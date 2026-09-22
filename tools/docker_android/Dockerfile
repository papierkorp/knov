# reuses the knov-dev image (go toolchain, swag/gotext, go mod cache, repo copy,
# git safe.directory) instead of duplicating those steps - see tools/docker_dev/Dockerfile.
# built via `make docker-prepare-android-image`, which builds knov-dev first
FROM knov-dev

# jdk + android sdk cmdline-tools: android.builder.sdkDownload=false in
# android/gradle.properties means AGP won't fetch platform/build-tools itself,
# they have to already be present via sdkmanager.
# openjdk-17 isn't packaged on this base image's debian release anymore, so this
# uses 21 instead - still within the range gradle/this AGP version understands,
# unlike a too-new JDK (see the mobile-apk comment in the root Makefile)
RUN apt-get update && apt-get install -y --no-install-recommends openjdk-21-jdk-headless unzip curl && \
    rm -rf /var/lib/apt/lists/*

# assumes an amd64 image (the common case for docker on a work pc); on an arm64
# host adjust to java-21-openjdk-arm64
ENV JAVA_HOME=/usr/lib/jvm/java-21-openjdk-amd64
ENV ANDROID_HOME=/opt/android-sdk
ENV PATH=$ANDROID_HOME/cmdline-tools/latest/bin:$ANDROID_HOME/platform-tools:$PATH
# downloading gradle here is the whole point of docker-build-apk, not something to gate on
ENV ALLOW_GRADLE_DOWNLOAD=1

RUN mkdir -p $ANDROID_HOME/cmdline-tools && \
    curl -fsSL https://dl.google.com/android/repository/commandlinetools-linux-15859902_latest.zip -o /tmp/cmdline-tools.zip && \
    unzip -q /tmp/cmdline-tools.zip -d $ANDROID_HOME/cmdline-tools && \
    mv $ANDROID_HOME/cmdline-tools/cmdline-tools $ANDROID_HOME/cmdline-tools/latest && \
    rm /tmp/cmdline-tools.zip && \
    yes | sdkmanager --licenses > /dev/null && \
    sdkmanager "platform-tools" "platforms;android-34" "build-tools;34.0.0" "build-tools;35.0.0"

# no COPY here: docker-build-apk always bind-mounts $(CURDIR) onto /app at runtime
# (see the Makefile), which shadows anything copied into the image at build time anyway
CMD ["make", "mobile-apk"]
