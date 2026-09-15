require 'minitest/autorun'
require 'tmpdir'
require 'fileutils'
require 'open3'
require_relative 'xcode-dependency-headers'

class XcodeDependencyHeadersTest < Minitest::Test
  WARNINGS = %w[everything].freeze

  def test_header_pragmas_are_idempotent_and_reversible
    ['#pragma once', "// License\nvoid example(void);\n", ''].each do |source|
      filtered = kinosail_dependency_header(source, WARNINGS, false)
      assert_includes filtered, source
      assert_equal filtered, kinosail_dependency_header(filtered, WARNINGS, false)
      assert_equal source, kinosail_dependency_header(filtered, WARNINGS, true)
      assert_equal source, kinosail_dependency_header(source, WARNINGS, true)
    end
  end

  def test_invalid_and_partial_headers_are_rejected
    [nil, {}, 'x' * (1024 * 1024 + 1), '// Kinosail dependency header diagnostics'].each do |source|
      assert_raises(RuntimeError) { kinosail_dependency_header(source, WARNINGS, false) }
    end
  end

  def test_files_are_validated_before_any_rewrite_and_unknown_pods_are_untouched
    Dir.mktmpdir do |root|
      first = File.join(root, 'Headers', 'VLCKit.h')
      second = File.join(root, 'Other', 'Headers', 'VLCKit.h')
      [first, second].each { |path| FileUtils.mkdir_p(File.dirname(path)) }
      original = 'void example(void);'
      File.write(first, original)
      File.write(second, 'x' * (1024 * 1024 + 1))
      pod = Struct.new(:label, :file_accessors) do
        def umbrella_header_path; nil; end
      end
      accessor = Struct.new(:root) do
        def public_headers(_frameworks); Dir.glob(File.join(root, '**/Headers/*.h')); end
      end
      fixture = pod.new('VLCKit', [accessor.new(root)])
      assert_raises(RuntimeError) { kinosail_dependency_headers([fixture], false) }
      assert_equal original, File.read(first)
      File.delete(second)
      kinosail_dependency_headers([pod.new('LocalModule', [accessor.new(root)])], false)
      assert_equal original, File.read(first)
      File.chmod(0o444, first)
      cached = File.join(root, 'Cached.h')
      File.link(first, cached)
      kinosail_dependency_headers([fixture], false)
      assert_equal 0o444, File.stat(first).mode & 0o777
      assert_includes File.read(first), '#pragma clang diagnostic push'
      assert_equal original, File.read(cached)
      assert_equal 0o444, File.stat(cached).mode & 0o777
      kinosail_dependency_headers([fixture], true)
      assert_equal original, File.read(first)
      assert_equal 0o444, File.stat(first).mode & 0o777
    end
  end

  def test_clang_keeps_client_diagnostics_and_dependency_errors_visible
    skip 'requires Apple clang' unless RUBY_PLATFORM.include?('darwin')
    Dir.mktmpdir do |root|
      header = File.join(root, 'Vendor.h')
      client = File.join(root, 'client.m')
      source = 'static inline int vendor_helper(void) { int unused_vendor; return 0; } __attribute__((deprecated("old API"))) int old_api(void);'
      File.write(header, source)
      File.write(File.join(root, 'module.modulemap'), "module Vendor {\nheader \"Vendor.h\"\nexport *\n}\n")
      File.write(client, "#include <Vendor.h>\nint app(void) { int unused_app; return old_api(); }\n")
      command = ['xcrun', 'clang', '-x', 'objective-c', '-fmodules', "-fmodules-cache-path=#{root}/cache", "-I#{root}", '-Wall', '-Wextra', '-fsyntax-only', client]
      _, audit, status = Open3.capture3(*command)
      assert status.success?, audit
      assert_includes audit, "warning: unused variable 'unused_vendor'"
      File.write(header, kinosail_dependency_header(source, WARNINGS, false))
      _, filtered, status = Open3.capture3(*command)
      assert status.success?, filtered
      refute_includes filtered, "warning: unused variable 'unused_vendor'"
      assert_includes filtered, "warning: unused variable 'unused_app'"
      assert_includes filtered, 'old API'
      File.write(header, kinosail_dependency_header("#error vendor_error\n", WARNINGS, false))
      _, errors, status = Open3.capture3(*command)
      refute status.success?
      assert_includes errors, 'vendor_error'
    end
  end

  def test_swift_unsigned_objc_api_imports_are_preserved
    skip 'requires Apple Swift' unless RUBY_PLATFORM.include?('darwin')
    Dir.mktmpdir do |root|
      source = "#import <Foundation/Foundation.h>\n@interface VendorCounter : NSObject\n@property (readonly) NSUInteger count;\n@end\n"
      File.write(File.join(root, 'Vendor.h'), kinosail_dependency_header(source, WARNINGS, false))
      File.write(File.join(root, 'module.modulemap'), "module Vendor {\nheader \"Vendor.h\"\nexport *\n}\n")
      client = File.join(root, 'client.swift')
      File.write(client, "import Vendor\nfunc count(_ value: VendorCounter) -> UInt { value.count }\n")
      _, diagnostics, status = Open3.capture3('xcrun', 'swiftc', '-typecheck', '-I', root, '-module-cache-path', File.join(root, 'cache'), client)
      assert status.success?, diagnostics
    end
  end
  def test_umbrella_metadata_warnings_are_scoped_and_reversible
    skip 'requires Apple Swift' unless RUBY_PLATFORM.include?('darwin')
    Dir.mktmpdir do |root|
      File.write(File.join(root, 'module.modulemap'), "module Vendor { umbrella header \"Vendor.h\" export * }\n")
      File.write(File.join(root, 'Extra.h'), "void extra(void);\n")
      client = File.join(root, 'client.swift')
      File.write(client, "import Vendor\n")
      original = "void example(void);\n"
      header = File.join(root, 'Vendor.h')
      File.write(header, original)
      command = ['xcrun', 'swiftc', '-typecheck', '-Xcc', '-Wincomplete-umbrella', '-I', root, '-module-cache-path', File.join(root, 'cache'), client]
      _, audit, status = Open3.capture3(*command)
      assert status.success?, audit
      assert_includes audit, 'umbrella header'
      filtered = kinosail_dependency_header(original, WARNINGS, false, umbrella: true)
      File.write(header, filtered)
      _, diagnostics, status = Open3.capture3(*command)
      assert status.success?, diagnostics
      refute_includes diagnostics, 'warning:'
      assert_equal original, kinosail_dependency_header(filtered, WARNINGS, true, umbrella: true)
    end
  end

end
