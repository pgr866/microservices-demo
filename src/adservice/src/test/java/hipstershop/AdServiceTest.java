package hipstershop;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.junit.jupiter.api.Assertions.fail;

import hipstershop.Demo.Ad;
import hipstershop.Demo.AdRequest;
import hipstershop.Demo.AdResponse;
import io.grpc.Server;
import io.grpc.health.v1.HealthCheckRequest;
import io.grpc.health.v1.HealthCheckResponse;
import io.grpc.health.v1.HealthCheckResponse.ServingStatus;
import io.grpc.health.v1.HealthGrpc;
import io.grpc.protobuf.services.HealthStatusManager;
import io.grpc.stub.StreamObserver;
import java.util.HashSet;
import java.util.List;
import java.util.Random;
import java.util.Set;
import java.util.concurrent.TimeUnit;
import org.apache.logging.log4j.Level;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.CsvSource;
import org.junit.jupiter.params.provider.ValueSource;

class AdServiceTest {

  private static final Set<String> CATALOG =
      Set.of(
          "/product/2ZYFJ3GM2N",
          "/product/66VCHSJNUP",
          "/product/0PUK6V6EV0",
          "/product/9SIQT8TOJO",
          "/product/1YMWWN1N4O",
          "/product/6E92ZMYYFZ",
          "/product/L9ECAV7KIM");

  private static List<String> getAds(AdService.AdServiceImpl service, String... contextKeys) {
    var observer = new CapturingObserver<AdResponse>();
    service.getAds(
        AdRequest.newBuilder().addAllContextKeys(List.of(contextKeys)).build(), observer);
    assertTrue(observer.completed, "the call must complete");
    return observer.response.getAdsList().stream().map(Ad::getRedirectUrl).toList();
  }

  private static List<String> getAds(String... contextKeys) {
    return getAds(new AdService.AdServiceImpl(), contextKeys);
  }

  private static final class CapturingObserver<T> implements StreamObserver<T> {
    T response;
    boolean completed;

    @Override
    public void onNext(T value) {
      response = value;
    }

    @Override
    public void onError(Throwable t) {
      fail("unexpected error", t);
    }

    @Override
    public void onCompleted() {
      completed = true;
    }
  }

  @Test
  void knownCategoryReturnsItsAd() {
    assertEquals(List.of("/product/66VCHSJNUP"), getAds("clothing"));
  }

  @Test
  void categoryWithSeveralAdsReturnsAllOfThem() {
    assertEquals(List.of("/product/9SIQT8TOJO", "/product/6E92ZMYYFZ"), getAds("kitchen"));
  }

  @Test
  void severalCategoriesReturnTheAdsOfEachInRequestOrder() {
    assertEquals(List.of("/product/2ZYFJ3GM2N", "/product/0PUK6V6EV0"), getAds("hair", "decor"));
  }

  @Test
  void unknownCategoriesAreIgnoredWhenAnotherOneMatches() {
    // The random fallback only kicks in when nothing matched at all.
    assertEquals(List.of("/product/66VCHSJNUP"), getAds("clothing", "not-a-category"));
  }

  @Test
  void categoriesAreCaseSensitive() {
    // Same as the original service: "Clothing" is not "clothing", so it gets random ads.
    assertEquals(AdService.MAX_ADS_TO_SERVE, getAds("Clothing").size());
  }

  @ParameterizedTest
  @ValueSource(strings = {"", "not-a-category"})
  void noMatchFallsBackToDifferentAdsFromTheCatalog(String key) {
    String[] keys = key.isEmpty() ? new String[0] : new String[] {key};
    var service = new AdService.AdServiceImpl(new Random(1));
    for (int i = 0; i < 50; i++) {
      List<String> ads = getAds(service, keys);
      assertEquals(AdService.MAX_ADS_TO_SERVE, ads.size());
      assertEquals(ads.size(), Set.copyOf(ads).size(), "the same ad twice: " + ads);
      assertTrue(CATALOG.containsAll(ads), "not in the catalog: " + ads);
    }
  }

  @Test
  void randomFallbackCoversTheWholeCatalog() {
    var service = new AdService.AdServiceImpl(new Random(1));
    Set<String> seen = new HashSet<>();
    for (int i = 0; i < 100; i++) {
      seen.addAll(getAds(service));
    }
    assertEquals(CATALOG, seen);
  }

  @Test
  void catalogHoldsEveryAdOnce() {
    assertEquals(CATALOG.size(), AdService.ADS.size());
    assertEquals(
        CATALOG, Set.copyOf(AdService.ADS.values().stream().map(Ad::getRedirectUrl).toList()));
  }

  @ParameterizedTest
  @CsvSource({"debug,DEBUG", "info,INFO", "Info,INFO", "warn,WARN", "ERROR,ERROR"})
  void logLevelAcceptsTheSameValuesAsTheOtherServices(String value, String level) {
    assertEquals(Level.getLevel(level), AdService.logLevel(value));
  }

  @Test
  void logLevelDefaultsToInfoWhenUnsetOrEmpty() {
    assertEquals(Level.INFO, AdService.logLevel(null));
    assertEquals(Level.INFO, AdService.logLevel(""));
  }

  @ParameterizedTest
  @ValueSource(strings = {"verbose", "trace", "fatal", "off", "all", " info"})
  void logLevelRejectsAnythingElse(String value) {
    // log4j itself takes trace, fatal, off and all: LOG_LEVEL=off would silence the service.
    assertThrows(IllegalArgumentException.class, () -> AdService.logLevel(value));
  }

  /** How the calls in flight end while the server drains them. */
  private enum Drain {
    IN_TIME,
    TOO_SLOW,
    INTERRUPTED
  }

  /** Records what stop() asks of the server, and the health status when draining starts. */
  private static final class FakeServer extends Server {
    private final Drain drain;
    private final HealthStatusManager health;
    ServingStatus statusWhenDraining;
    boolean cut;

    FakeServer(Drain drain, HealthStatusManager health) {
      this.drain = drain;
      this.health = health;
    }

    @Override
    public Server start() {
      return this;
    }

    @Override
    public Server shutdown() {
      statusWhenDraining = servingStatus(health);
      return this;
    }

    @Override
    public Server shutdownNow() {
      cut = true;
      return this;
    }

    @Override
    public boolean isShutdown() {
      return statusWhenDraining != null;
    }

    @Override
    public boolean isTerminated() {
      return isShutdown();
    }

    @Override
    public boolean awaitTermination(long timeout, TimeUnit unit) throws InterruptedException {
      if (drain == Drain.INTERRUPTED) {
        throw new InterruptedException();
      }
      return drain == Drain.IN_TIME;
    }

    @Override
    public void awaitTermination() {}
  }

  private static ServingStatus servingStatus(HealthStatusManager health) {
    var observer = new CapturingObserver<HealthCheckResponse>();
    ((HealthGrpc.HealthImplBase) health.getHealthService())
        .check(HealthCheckRequest.getDefaultInstance(), observer);
    return observer.response.getStatus();
  }

  @Test
  void stopReportsNotServingBeforeDraining() {
    var health = new HealthStatusManager();
    var server = new FakeServer(Drain.IN_TIME, health);

    AdService.stop(server, health, 1, TimeUnit.SECONDS);

    assertEquals(ServingStatus.NOT_SERVING, server.statusWhenDraining);
    assertFalse(server.cut, "calls that finish in time must not be cut");
  }

  @Test
  void stopCutsTheCallsStillInFlightAfterTheTimeout() {
    var health = new HealthStatusManager();
    var server = new FakeServer(Drain.TOO_SLOW, health);

    AdService.stop(server, health, 1, TimeUnit.MILLISECONDS);

    assertTrue(server.cut);
  }

  @Test
  void stopCutsTheCallsAndKeepsTheInterruptIfInterruptedWhileDraining() {
    var health = new HealthStatusManager();
    var server = new FakeServer(Drain.INTERRUPTED, health);

    AdService.stop(server, health, 1, TimeUnit.SECONDS);

    assertTrue(server.cut);
    // Thread.interrupted() also clears the flag, so the next tests run uninterrupted.
    assertTrue(Thread.interrupted(), "the interrupt must be kept for the caller");
  }
}
