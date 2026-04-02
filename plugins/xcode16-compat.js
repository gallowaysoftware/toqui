/**
 * Expo config plugin: Xcode 16.4 compatibility shims.
 *
 * Expo SDK 55 officially requires Xcode 26 (Swift 6.2 / iOS 26 SDK).
 * This plugin injects a Podfile post_install hook that:
 *
 *   1. Forces every pod to Swift 5 language mode so strict-concurrency
 *      violations (hard errors in Swift 6) become warnings.
 *   2. Sets SWIFT_STRICT_CONCURRENCY to 'minimal'.
 *
 * The pnpm patches in patches/ handle the remaining source-level
 * incompatibilities (@MainActor conformance syntax, iOS 26 APIs).
 *
 * Remove this plugin once the project migrates to Xcode 26.
 */
const { withDangerousMod } = require("expo/config-plugins");
const fs = require("fs");
const path = require("path");

const POST_INSTALL_SNIPPET = `
    # ── Xcode 16.4 compatibility shims (injected by plugins/xcode16-compat.js) ──
    installer.pods_project.targets.each do |target|
      target.build_configurations.each do |bc|
        bc.build_settings['SWIFT_VERSION'] = '5'
        bc.build_settings['SWIFT_STRICT_CONCURRENCY'] = 'minimal'
        flags = bc.build_settings['OTHER_SWIFT_FLAGS'] || '$(inherited)'
        bc.build_settings['OTHER_SWIFT_FLAGS'] = "\#{flags} -Xfrontend -strict-concurrency=minimal"
      end
    end`;

function withXcode16Compat(config) {
  return withDangerousMod(config, [
    "ios",
    async (cfg) => {
      const podfilePath = path.join(
        cfg.modRequest.platformProjectRoot,
        "Podfile"
      );
      let podfile = fs.readFileSync(podfilePath, "utf8");

      // Only inject once
      if (podfile.includes("Xcode 16.4 compatibility shims")) {
        return cfg;
      }

      // Insert before the closing `end` of the post_install block
      // The pattern: find the last `  end\nend` which closes post_install + target
      podfile = podfile.replace(
        /(\n\s*end\s*\nend\s*)$/,
        `\n${POST_INSTALL_SNIPPET}\n$1`
      );

      fs.writeFileSync(podfilePath, podfile);
      return cfg;
    },
  ]);
}

module.exports = withXcode16Compat;
