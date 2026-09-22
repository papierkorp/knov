plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

android {
    namespace = "com.knov.wrapper"
    compileSdk = 34

    defaultConfig {
        applicationId = "com.knov.wrapper"
        minSdk = 26
        targetSdk = 34
        versionCode = 1
        versionName = "1.0"
    }

    // keeps the debug build's signature stable across machines/CI runs (default gradle
    // behaviour is an ephemeral per-environment debug.keystore), so a sideloaded APK from
    // one release can be upgraded in place via `adb install -r` from the next one
    signingConfigs {
        getByName("debug") {
            storeFile = file("debug.keystore")
            storePassword = "android"
            keyAlias = "androiddebugkey"
            keyPassword = "android"
        }
    }

    // no AAB/Play Store path and no real release signing config: distribution is a debug-signed
    // sideloadable APK only (see the debug signingConfig above and `make mobile-apk`) - deliberate,
    // not an oversight, since this is a self-hosted personal tool, not a store-published app
    buildTypes {
        release {
            isMinifyEnabled = false
        }
    }

    // force the bundled go binary (packaged as jniLibs/*/libknovserver.so) to be
    // extracted to a real, executable path on disk instead of mapped from inside
    // the apk - it's spawned as a subprocess, not dlopen'd as an actual library.
    packaging {
        jniLibs {
            useLegacyPackaging = true
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
    kotlinOptions {
        jvmTarget = "17"
    }
}

dependencies {
    implementation("androidx.core:core-ktx:1.13.1")
    implementation("androidx.appcompat:appcompat:1.7.0")
}
