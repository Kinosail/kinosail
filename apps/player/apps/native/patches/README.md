# Legacy native build patches

These patches belong to the retained Expo/React Native implementation. They are not compiled into the active [Swift clients](../README.md) and are not required for their builds.

These patches apply through `pnpm install --frozen-lockfile`. Review and remove a patch when its upstream release contains the fix; changing dependency versions must not silently skip patches.

- Gesture Handler: give the delayed pan callback its own selector, preserving the delay property's getter; use the UIKit direction type; mark the header helper inline; make an existing block capture explicit.
- Worklets / React Native: retain deprecation guidance as prose instead of a compiler documentation tag without a matching attribute. Inline header-only queue helpers.
- Reanimated: describe callback nullability and the nullable weak presenter; inline header-only queue helpers.
- Expo Video: keep fullscreen and picture-in-picture view ownership with AVKit when inline bounds or safe-area insets change. Requires a native rebuild; verify play, rotation, PiP, and fullscreen exit on iPhone.
- Expo FileSystem: correct the upload-task cast, check the callback task's class, and describe NSURLSession nullability. Restate exception subclasses' existing inherited unchecked Sendable contract. The legacy source fixes apply where those sources are built; packaged precompiled frameworks retain their upstream binary implementation.
- Expo Modules JSI: forward the parent Xcode warning settings into its isolated framework build; app settings are not changed.
- Expo Modules Core: import the implemented React protocol and describe file-system interface nullability.
- Expo: restate inherited exception Sendable conformance; evaluate fetched Metro bundles explicitly in global scope, matching Hermes semantics.
- Safe Area Context: initialize generated inset fields in their C++ declaration order without changing values.
- Screens: include tvOS in the trait-override availability guard and supply no-op scroll-to-top guards on platforms without that interaction.
- React Native's Xcode bundle script: accept a compiler-only globals file, preserving undeclared-name diagnostics for other identifiers. The declarations do not execute or replace runtime globals.

The Kinosail Xcode config plugin removes a duplicate explicit libc++ linker flag, aligns the bundler's color environment, and marks the already-unconditional Hermes configuration script accordingly. External CocoaPods targets (all source roots under `node_modules` or `Pods`) filter compiler and linker warnings, while local modules, generated app code, and unknown/mixed source roots retain their diagnostics. Public headers from the noisy VLCKit, Expo, React, and JSI dependencies use scoped diagnostic pragmas; incomplete-umbrella metadata warnings are disabled at the external umbrella headers. Module system-header annotations are deliberately avoided because they change Swift's Objective-C integer imports. The audit switch reverses the header pragmas. Compiler errors and deprecated API use in app code remain visible. Known platform/generated empty translation units omit libtool's no-symbol warning. These filters do not fix underlying upstream diagnostics.

To audit upstream compiler warnings, run `KINOSAIL_DEPENDENCY_WARNINGS=1 pod install` in the generated `ios` directory, then clean-build with Xcode. Regenerate pods without that variable to restore dependency filtering. The compiler-only Hermes globals declarations remain active in either mode.

Focused checks (run directly only when authorized while `.gates-disabled` exists):

```sh
node --test scripts/test-expo-bundle-loader.cjs
ruby plugins/xcode-build-settings.test.rb
ruby plugins/xcode-dependency-headers.test.rb
pnpm exec jest --runInBand plugins/with-xcode-build-cleanup.test.js scripts/hermes-globals.test.js
```

## Earlier build evidence (2026-09-08, before dependency filtering)

Fresh ARM64 Release simulator builds with separate empty DerivedData directories succeeded with zero errors for iOS and tvOS. Warning occurrences (including repeated header diagnostics) were 1,052 before and 822 after on iOS; 614 remaining occurrences concern incomplete React/JSI umbrella headers. tvOS completed with 1,328 occurrences, including 918 umbrella-header diagnostics; no comparable clean tvOS baseline was captured. These counts do not represent unique defects. Both signed simulator apps installed, launched, and rendered the server setup screen.

Remaining dependency diagnostics include legacy React APIs, header nullability/protocol declarations, Swift concurrency declarations, numeric narrowing, and App Intents metadata extraction. Precompiled framework headers were not rewritten and warning categories were not globally disabled. This is build and startup evidence only; automated suites remain unrun under `.gates-disabled`, and physical-device, playback, and App Store archive verification were not repeated for these patches.
