plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.plugin.compose")
}

android {
    namespace = "com.kinosail.player"
    compileSdk = 37

    defaultConfig {
        applicationId = "com.kinosail.player"
        minSdk = 23
        targetSdk = 36
        versionCode = 1
        versionName = "0.1.0"
    }

    buildTypes {
        getByName("debug") {
            applicationIdSuffix = ".dev"
            versionNameSuffix = "-dev"
        }
    }

    buildFeatures { compose = true }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
}

dependencies {
    implementation(platform("androidx.compose:compose-bom:2026.09.00"))
    implementation("androidx.activity:activity-compose:1.13.0")
    implementation("androidx.compose.material3:material3")
    implementation("androidx.tv:tv-material:1.1.0")
    implementation("androidx.lifecycle:lifecycle-viewmodel-compose:2.11.0")
    implementation("org.jetbrains.kotlinx:kotlinx-serialization-json:1.11.0")
    implementation("com.fasterxml.jackson.core:jackson-core:2.22.3")
    testImplementation("junit:junit:4.13.2")
}
