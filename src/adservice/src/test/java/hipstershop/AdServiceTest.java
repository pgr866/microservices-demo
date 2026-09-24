package hipstershop;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

import hipstershop.Demo.Ad;
import hipstershop.Demo.AdRequest;
import hipstershop.Demo.AdResponse;
import io.grpc.stub.StreamObserver;
import java.util.HashSet;
import java.util.List;
import java.util.Set;
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
  void getAds_withMultipleContextKeys_aggregatesAdsFromEachCategory() {
    AdService.AdServiceImpl service = new AdService.AdServiceImpl();
    CapturingObserver observer = new CapturingObserver();

    service.getAds(
        AdRequest.newBuilder().addContextKeys("hair").addContextKeys("decor").build(), observer);

    List<Ad> ads = observer.response.getAdsList();
    assertEquals(2, ads.size());
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
}
