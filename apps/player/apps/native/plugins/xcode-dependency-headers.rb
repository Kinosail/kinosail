# Silence imported header diagnostics only for the known noisy dependencies. Unlike system-module
# annotations, diagnostic pragmas preserve Swift's Objective-C type imports.
def kinosail_dependency_header(contents, warnings, audit, umbrella: false)
  raise 'Invalid dependency header' unless contents.is_a?(String) && contents.bytesize <= 1024 * 1024

  marker = '// Kinosail dependency header diagnostics'
  prefix = "#{marker}\n#pragma clang diagnostic push\n" + warnings.map { |warning| "#pragma clang diagnostic ignored \"-W#{warning}\"\n" }.join
  suffix = "\n#pragma clang diagnostic pop\n"
  # Clang checks umbrella completeness at EOF, after the diagnostic pop.
  suffix += "#pragma clang diagnostic ignored \"-Wincomplete-umbrella\"\n" if umbrella
  suffix += "// End Kinosail dependency header diagnostics\n"
  if contents.include?(marker)
    raise 'Incomplete dependency header diagnostics' unless contents.start_with?(prefix) && contents.end_with?(suffix)

    return audit ? contents.delete_prefix(prefix).delete_suffix(suffix) : contents
  end
  audit ? contents : prefix + contents + suffix
end

def kinosail_dependency_headers(pods, audit)
  names = %w[VLCKit Expo ExpoModulesCore ExpoFileSystem React-Core React-Core-prebuilt React-jsi React-RCTAppDelegate]
  paths = pods.select { |pod| names.include?(pod.label) }.flat_map do |pod|
    [pod.umbrella_header_path] + pod.file_accessors.flat_map { |accessor| accessor.public_headers(true) }
  end.compact.select { |path| File.file?(path) }.map { |path| File.realpath(path) }.uniq
  raise 'Too many dependency headers' if paths.length > 8192

  updates = paths.filter_map do |path|
    original = File.read(path, 1024 * 1024 + 1)
    umbrella = path.end_with?('-umbrella.h', '/jsi.h')
    updated = kinosail_dependency_header(original, %w[everything], audit, umbrella: umbrella)
    [path, updated] unless original == updated
  end
  updates.each do |path, contents|
    # Replace rather than modify an inode shared with a package cache. This
    # also handles CocoaPods' read-only headers without changing their modes.
    mode = File.stat(path).mode & 0o777
    Tempfile.create(['.kinosail-header-', '.tmp'], File.dirname(path)) do |file|
      file.write(contents)
      file.flush
      file.chmod(mode)
      File.rename(file.path, path)
    end
  end
end
require 'tempfile'
