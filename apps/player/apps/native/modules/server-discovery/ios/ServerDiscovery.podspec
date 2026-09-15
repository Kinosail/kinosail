Pod::Spec.new do |s|
  s.name = 'ServerDiscovery'
  s.version = '1.0.0'
  s.summary = 'Local Kinosail Player server discovery'
  s.license = { :type => 'LicenseRef-Kinosail' }
  s.author = 'Kinosail'
  s.homepage = 'https://kinosail.com'
  s.source = { :git => 'https://github.com/MikeO7/kinosail.git' }
  s.platforms = { :ios => '16.4', :tvos => '16.4' }
  s.swift_version = '5.9'
  s.static_framework = true
  s.dependency 'ExpoModulesCore'
  s.source_files = '**/*.swift'
end
