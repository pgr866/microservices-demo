package hipstershop;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.junit.jupiter.api.Assertions.fail;

import hipstershop.Demo.Ad;
import hipstershop.Demo.AdRequest;
import hipstershop.Demo.AdResponse;
import io.grpc.stub.StreamObserver;
import java.util.HashSet;
import java.util.List;
import java.util.Random;
import java.util.Set;
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
    var observer = new CapturingObserver();
    service.getAds(
        AdRequest.newBuilder().addAllContextKeys(List.of(contextKeys)).build(), observer);
    assertTrue(observer.completed, "the call must complete");
    return observer.response.getAdsList().stream().map(Ad::getRedirectUrl).toList();
  }

  private static List<String> getAds(String... contextKeys) {
    return getAds(new AdService.AdServiceImpl(), contextKeys);
  }

  private static final class CapturingObserver implements StreamObserver<AdResponse> {
    AdResponse response;
    boolean completed;

    @Override
    public void onNext(AdResponse value) {
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
}
