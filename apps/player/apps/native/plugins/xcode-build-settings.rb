require_relative 'xcode-dependency-headers'

# Keep app/local-module diagnostics. Set KINOSAIL_DEPENDENCY_WARNINGS=1 during
# pod install to include external dependency warnings in an upstream audit.
def kinosail_xcode_build_settings(installer)
  audit = ENV.fetch('KINOSAIL_DEPENDENCY_WARNINGS', '0')
  raise 'KINOSAIL_DEPENDENCY_WARNINGS must be 0 or 1' unless %w[0 1].include?(audit)

  external_pods = installer.pod_targets.select do |pod|
    roots = pod.file_accessors.map(&:root)
    !roots.empty? && roots.all? do |root|
      path = root&.realpath&.to_s
      path && (path.include?('/node_modules/') || path.start_with?("#{installer.sandbox.root.realpath}/"))
    rescue SystemCallError
      false
    end
  end
  kinosail_dependency_headers(external_pods, audit == '1')
  external_targets = external_pods.map(&:label)
  installer.pods_project.targets.each do |target|
    if external_targets.include?(target.name)
      target.build_configurations.each do |config|
        config.build_settings['GCC_WARN_INHIBIT_ALL_WARNINGS'] = audit == '1' ? 'NO' : 'YES'
        config.build_settings['SWIFT_SUPPRESS_WARNINGS'] = audit == '1' ? 'NO' : 'YES'
        config.build_settings['LD_SUPPRESS_WARNINGS'] = audit == '1' ? 'NO' : 'YES'
      end
    end
    # These dependencies intentionally compile empty translation units for
    # generated components or platform/Release-only preprocessor branches.
    if %w[ReactCodegen ExpoLogBox react-native-safe-area-context RNScreens GTMSessionFetcher react-native-google-cast RNWorklets RNReanimated ExpoFileSystem libdav1d SDWebImage SDWebImageWebPCoder].include?(target.name)
      target.build_configurations.each do |config|
        flags = Array(config.build_settings['OTHER_LIBTOOLFLAGS'] || '$(inherited)')
        config.build_settings['OTHER_LIBTOOLFLAGS'] = flags | ['-no_warning_for_no_symbols']
      end
    end
    next unless target.name == 'hermes-engine'

    target.shell_script_build_phases.each do |phase|
      next unless phase.name == '[CP-User] [Hermes] Replace Hermes for the right configuration, if needed'

      # The script must inspect the selected configuration on every build.
      phase.always_out_of_date = '1'
    end
  end
end
