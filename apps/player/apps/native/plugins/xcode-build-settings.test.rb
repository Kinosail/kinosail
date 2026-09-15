require 'minitest/autorun'
require 'pathname'
require 'tmpdir'
require 'fileutils'
require 'open3'
require_relative 'xcode-build-settings'

class XcodeBuildSettingsTest < Minitest::Test
  Config = Struct.new(:build_settings)
  Phase = Struct.new(:name, :always_out_of_date)
  Target = Struct.new(:name, :build_configurations, :shell_script_build_phases)
  Project = Struct.new(:targets)
  Accessor = Struct.new(:root)
  Pod = Struct.new(:label, :file_accessors) do
    def umbrella_header_path; nil; end
  end
  Sandbox = Struct.new(:root)
  Installer = Struct.new(:pods_project, :pod_targets, :sandbox) do
    def initialize(project, pods = [])
      super(project, pods, Sandbox.new(Pathname('/app/ios/Pods')))
    end
  end

  def test_only_expected_empty_dependency_objects_are_quieted
    %w[ReactCodegen RNScreens GTMSessionFetcher react-native-google-cast RNWorklets RNReanimated ExpoFileSystem ExpoLogBox react-native-safe-area-context libdav1d SDWebImage SDWebImageWebPCoder].each do |name|
      config = Config.new({ 'OTHER_LIBTOOLFLAGS' => ['$(inherited)', '-existing'], 'OTHER_CFLAGS' => '-Wall', 'SWIFT_SUPPRESS_WARNINGS' => 'NO' })
      installer = Installer.new(Project.new([Target.new(name, [config], [])]))
      2.times { kinosail_xcode_build_settings(installer) }
      assert_equal ['$(inherited)', '-existing', '-no_warning_for_no_symbols'], config.build_settings['OTHER_LIBTOOLFLAGS']
      assert_equal '-Wall', config.build_settings['OTHER_CFLAGS']
      assert_equal 'NO', config.build_settings['SWIFT_SUPPRESS_WARNINGS']
    end
  end

  def test_local_modules_and_other_dependencies_keep_all_settings
    %w[ProtectedMedia ServerDiscovery KinosailPlayer UnrelatedPod].each do |name|
      settings = { 'OTHER_LIBTOOLFLAGS' => '-existing', 'OTHER_CFLAGS' => '-Wall' }
      config = Config.new(settings.dup)
      kinosail_xcode_build_settings(Installer.new(Project.new([Target.new(name, [config], [])])))
      assert_equal settings, config.build_settings
    end
  end

  def test_missing_flags_inherit_and_hermes_marks_only_its_unconditional_script
    config = Config.new({})
    phase = Phase.new('[CP-User] [Hermes] Replace Hermes for the right configuration, if needed')
    other = Phase.new('Other script')
    targets = [Target.new('ReactCodegen', [config], []), Target.new('hermes-engine', [], [phase, other])]
    kinosail_xcode_build_settings(Installer.new(Project.new(targets)))
    assert_equal ['$(inherited)', '-no_warning_for_no_symbols'], config.build_settings['OTHER_LIBTOOLFLAGS']
    assert_equal '1', phase.always_out_of_date
    assert_nil other.always_out_of_date
  end
  def test_only_proven_external_sources_are_filtered_and_audit_restores_warnings
    previous = ENV['KINOSAIL_DEPENDENCY_WARNINGS']
    Dir.mktmpdir do |root|
      paths = %w[node_modules/expo ios/Pods/SDWebImage modules/custom ios/build/generated ios/Pods-ours].map { |path| File.join(root, path) }
      paths.each { |path| FileUtils.mkdir_p(path) }
      local_link = File.join(root, 'node_modules/local-link')
      File.symlink(paths[2], local_link)
      paths += [nil, local_link, File.join(root, 'node_modules/missing')]
      targets = paths.each_index.map { |i| Target.new("Pod#{i}", [Config.new({})], []) }
      pods = paths.each_with_index.map { |path, i| Pod.new("Pod#{i}", [Accessor.new(path && Pathname(path))]) }
      pods << Pod.new('Mixed', [Accessor.new(Pathname(paths[0])), Accessor.new(Pathname(paths[2]))])
      pods << Pod.new('Unknown', [])
      targets += %w[Mixed Unknown].map { |name| Target.new(name, [Config.new({})], []) }
      installer = Installer.new(Project.new(targets), pods)
      installer.sandbox.root = Pathname(File.join(root, 'ios/Pods'))
      %w[0 1].each do |audit|
        ENV['KINOSAIL_DEPENDENCY_WARNINGS'] = audit
        kinosail_xcode_build_settings(installer)
        targets.each_with_index do |target, i|
          settings = target.build_configurations.first.build_settings
          if i < 2
            assert_equal(audit == '1' ? 'NO' : 'YES', settings['GCC_WARN_INHIBIT_ALL_WARNINGS'])
            assert_equal(audit == '1' ? 'NO' : 'YES', settings['SWIFT_SUPPRESS_WARNINGS'])
            assert_equal(audit == '1' ? 'NO' : 'YES', settings['LD_SUPPRESS_WARNINGS'])
          else
            assert_empty settings
          end
        end
      end
    end
  ensure
    ENV['KINOSAIL_DEPENDENCY_WARNINGS'] = previous
  end

  def test_invalid_audit_setting_rejects_before_any_mutation
    config = Config.new({})
    installer = Installer.new(Project.new([Target.new('ReactCodegen', [config], [])]))
    previous = ENV['KINOSAIL_DEPENDENCY_WARNINGS']
    ['', 'true', '2', '1 ', 'x' * 4096].each do |value|
      ENV['KINOSAIL_DEPENDENCY_WARNINGS'] = value
      assert_raises(RuntimeError) { kinosail_xcode_build_settings(installer) }
      assert_empty config.build_settings
    end
  ensure
    ENV['KINOSAIL_DEPENDENCY_WARNINGS'] = previous
  end

  def test_dependency_linker_warning_filter_does_not_hide_undefined_symbols
    skip 'requires Apple clang' unless RUBY_PLATFORM.include?('darwin')
    Dir.mktmpdir do |root|
      source = File.join(root, 'missing.c')
      File.write(source, 'extern int missing_dependency_symbol(void); int main(void) { return missing_dependency_symbol(); }')
      _, diagnostics, status = Open3.capture3('xcrun', 'clang', source, '-Wl,-w', '-o', File.join(root, 'output'))
      refute status.success?
      assert_includes diagnostics, 'missing_dependency_symbol'
      refute File.exist?(File.join(root, 'output'))
    end
  end

end
