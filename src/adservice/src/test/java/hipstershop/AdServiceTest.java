package hipstershop;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import hipstershop.Demo.Ad;
import hipstershop.Demo.AdRequest;
import hipstershop.Demo.AdResponse;
import io.grpc.stub.StreamObserver;
import java.util.HashSet;
import java.util.List;
import java.util.Set;
import org.apache.logging.log4j.Level;
import org.junit.jupiter.api.Test;

class AdServiceTest {

  /** Captures the single response a unary gRPC call produces, without a real network call. */
  private static class CapturingObserver implements StreamObserver<AdResponse> {
    AdResponse response;
    Throwable error;

    @Override
    public void onNext(AdResponse value) {
      response = value;
    }

    @Override
    public void onError(Throwable t) {
      error = t;
    }

    @Override
    public void onCompleted() {}
  }

  @Test
  void getAds_withKnownContextKey_returnsAdsForThatCategory() {
    AdService.AdServiceImpl service = new AdService.AdServiceImpl();
    CapturingObserver observer = new CapturingObserver();

    service.getAds(AdRequest.newBuilder().addContextKeys("clothing").build(), observer);

    List<Ad> ads = observer.response.getAdsList();
    assertEquals(1, ads.size());
    assertEquals("/product/66VCHSJNUP", ads.get(0).getRedirectUrl());
  }

  @Test
  void getAds_withCategoryHoldingSeveralAds_returnsAllOfThem() {
    AdService.AdServiceImpl service = new AdService.AdServiceImpl();
    CapturingObserver observer = new CapturingObserver();

    service.getAds(AdRequest.newBuilder().addContextKeys("kitchen").build(), observer);

    Set<String> urls = new HashSet<>();
    observer.response.getAdsList().forEach(ad -> urls.add(ad.getRedirectUrl()));
    assertEquals(Set.of("/product/9SIQT8TOJO", "/product/6E92ZMYYFZ"), urls);
  }

  @Test
  void getAds_withMultipleContextKeys_aggregatesAdsFromEachCategory() {
    AdService.AdServiceImpl service = new AdService.AdServiceImpl();
    CapturingObserver observer = new CapturingObserver();

    service.getAds(
        AdRequest.newBuilder().addContextKeys("hair").addContextKeys("decor").build(), observer);

    List<Ad> ads = observer.response.getAdsList();
    assertEquals(2, ads.size());
  }

  @Test
  void getAds_withKnownAndUnknownContextKeys_returnsOnlyMatchingAdsWithoutRandomFill() {
    AdService.AdServiceImpl service = new AdService.AdServiceImpl();
    CapturingObserver observer = new CapturingObserver();

    service.getAds(
        AdRequest.newBuilder()
            .addContextKeys("clothing")
            .addContextKeys("not-a-real-category")
            .build(),
        observer);

    // The random fallback only kicks in when nothing matched at all.
    List<Ad> ads = observer.response.getAdsList();
    assertEquals(1, ads.size());
    assertEquals("/product/66VCHSJNUP", ads.get(0).getRedirectUrl());
  }

  @Test
  void getAds_withUnknownContextKey_fallsBackToRandomAds() {
    AdService.AdServiceImpl service = new AdService.AdServiceImpl();
    CapturingObserver observer = new CapturingObserver();

    service.getAds(AdRequest.newBuilder().addContextKeys("not-a-real-category").build(), observer);

    // No ads match the category, so the service falls back to MAX_ADS_TO_SERVE random ads.
    assertEquals(2, observer.response.getAdsList().size());
  }

  @Test
  void getAds_withNoContextKeys_returnsRandomAds() {
    AdService.AdServiceImpl service = new AdService.AdServiceImpl();
    CapturingObserver observer = new CapturingObserver();

    service.getAds(AdRequest.newBuilder().build(), observer);

    assertEquals(2, observer.response.getAdsList().size());
  }

  @Test
  void getAds_calledRepeatedlyWithNoContextKeys_variesAcrossTheFullCatalog() {
    AdService.AdServiceImpl service = new AdService.AdServiceImpl();
    Set<String> seenUrls = new HashSet<>();

    for (int i = 0; i < 50; i++) {
      CapturingObserver observer = new CapturingObserver();
      service.getAds(AdRequest.newBuilder().build(), observer);
      observer.response.getAdsList().forEach(ad -> seenUrls.add(ad.getRedirectUrl()));
    }

    // 7 ads exist in total; 50 random draws of 2 should have surfaced more than just one.
    assertTrue(seenUrls.size() > 1);
  }

  @Test
  void logLevelAcceptsTheSameValuesAsTheOtherServices() {
    assertEquals(Level.INFO, AdService.logLevel(null));
    assertEquals(Level.INFO, AdService.logLevel(""));
    assertEquals(Level.DEBUG, AdService.logLevel("debug"));
    assertEquals(Level.WARN, AdService.logLevel("warn"));
    assertEquals(Level.ERROR, AdService.logLevel("ERROR"));
    assertThrows(IllegalArgumentException.class, () -> AdService.logLevel("verbose"));
  }
}
