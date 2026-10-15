/*
 * Copyright 2018, Google LLC.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package hipstershop;

import com.google.common.collect.ImmutableListMultimap;
import hipstershop.Demo.Ad;
import hipstershop.Demo.AdRequest;
import hipstershop.Demo.AdResponse;
import io.grpc.Server;
import io.grpc.ServerBuilder;
import io.grpc.health.v1.HealthCheckResponse.ServingStatus;
import io.grpc.protobuf.services.HealthStatusManager;
import io.grpc.stub.StreamObserver;
import java.io.IOException;
import java.util.ArrayList;
import java.util.Collections;
import java.util.List;
import java.util.Locale;
import java.util.Random;
import java.util.concurrent.TimeUnit;
import org.apache.logging.log4j.Level;
import org.apache.logging.log4j.LogManager;
import org.apache.logging.log4j.Logger;
import org.apache.logging.log4j.core.config.Configurator;

public final class AdService {

  private static final Logger logger = LogManager.getLogger(AdService.class);

  static final int MAX_ADS_TO_SERVE = 2;

  /** Below the 30 s Kubernetes waits before SIGKILL. */
  private static final long SHUTDOWN_TIMEOUT_SECONDS = 10;

  private Server server;
  private HealthStatusManager healthMgr;

  private void start(int port) throws IOException {
    healthMgr = new HealthStatusManager();
    server =
        ServerBuilder.forPort(port)
            .addService(new AdServiceImpl())
            .addService(healthMgr.getHealthService())
            .build()
            .start();
    logger.info("Ad Service started, listening on " + port);
    Runtime.getRuntime()
        .addShutdownHook(
            new Thread(
                () -> {
                  // log4j2.xml disables log4j's own shutdown hook, which could stop logging
                  // before this one ends: logging is stopped here, once the server is down.
                  logger.info("shutting down gRPC ads server since JVM is shutting down");
                  stop();
                  logger.info("server shut down");
                  LogManager.shutdown();
                }));
    healthMgr.setStatus("", ServingStatus.SERVING);
  }

  private void stop() {
    stop(server, healthMgr, SHUTDOWN_TIMEOUT_SECONDS, TimeUnit.SECONDS);
  }

  /** Reports NOT_SERVING, drains in-flight calls for up to the timeout, then cuts the rest. */
  static void stop(Server server, HealthStatusManager health, long timeout, TimeUnit unit) {
    health.enterTerminalState();
    server.shutdown();
    try {
      if (!server.awaitTermination(timeout, unit)) {
        logger.warn("calls still in flight, cutting them");
        server.shutdownNow();
      }
    } catch (InterruptedException e) {
      server.shutdownNow();
      Thread.currentThread().interrupt();
    }
  }

  /** The grpc library uses daemon threads, so the main thread has to wait. */
  private void blockUntilShutdown() throws InterruptedException {
    if ((server != null) == true) {
      server.awaitTermination();
    }
  }

  static final class AdServiceImpl extends AdServiceGrpc.AdServiceImplBase {

    private final Random random;

    AdServiceImpl() {
      this(new Random());
    }

    /** Takes the source of the random fallback, so tests can make it repeatable. */
    AdServiceImpl(Random random) {
      this.random = random;
    }

    @Override
    public void getAds(AdRequest req, StreamObserver<AdResponse> responseObserver) {
      logger.debug("received ad request (context_words=" + req.getContextKeysList() + ")");
      List<Ad> ads =
          req.getContextKeysList().stream().flatMap(key -> ADS.get(key).stream()).toList();
      if (ads.isEmpty()) {
        ads = randomAds();
      }
      responseObserver.onNext(AdResponse.newBuilder().addAllAds(ads).build());
      responseObserver.onCompleted();
    }

    private List<Ad> randomAds() {
      List<Ad> all = new ArrayList<>(ADS.values());
      Collections.shuffle(all, random);
      return all.subList(0, MAX_ADS_TO_SERVE);
    }
  }

  static final ImmutableListMultimap<String, Ad> ADS =
      ImmutableListMultimap.<String, Ad>builder()
          .put("clothing", ad("/product/66VCHSJNUP", "Tank top for sale. 20% off."))
          .put(
              "accessories",
              ad("/product/1YMWWN1N4O", "Watch for sale. Buy one, get second kit for free"))
          .put(
              "footwear",
              ad("/product/L9ECAV7KIM", "Loafers for sale. Buy one, get second one for free"))
          .put("hair", ad("/product/2ZYFJ3GM2N", "Hairdryer for sale. 50% off."))
          .put("decor", ad("/product/0PUK6V6EV0", "Candle holder for sale. 30% off."))
          .putAll(
              "kitchen",
              ad("/product/9SIQT8TOJO", "Bamboo glass jar for sale. 10% off."),
              ad("/product/6E92ZMYYFZ", "Mug for sale. Buy two, get third one for free"))
          .build();

  private static Ad ad(String redirectUrl, String text) {
    return Ad.newBuilder().setRedirectUrl(redirectUrl).setText(text).build();
  }

  static Level logLevel(String value) {
    if (value == null || value.isEmpty()) {
      return Level.INFO;
    }
    // Not Level.getLevel: it also takes trace, fatal, off and all, which no service accepts.
    return switch (value.toLowerCase(Locale.ROOT)) {
      case "debug" -> Level.DEBUG;
      case "info" -> Level.INFO;
      case "warn" -> Level.WARN;
      case "error" -> Level.ERROR;
      default ->
          throw new IllegalArgumentException(
              "invalid LOG_LEVEL \"" + value + "\": want debug, info, warn or error");
    };
  }

  public static void main(String[] args) throws IOException, InterruptedException {
    try {
      Configurator.setRootLevel(logLevel(System.getenv("LOG_LEVEL")));
    } catch (IllegalArgumentException e) {
      logger.fatal(e.getMessage());
      System.exit(1);
    }
    logger.info("AdService starting.");
    AdService service = new AdService();
    service.start(Integer.parseInt(System.getenv().getOrDefault("PORT", "9555")));
    service.blockUntilShutdown();
  }
}
