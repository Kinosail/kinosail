Pod::Spec.new do |s|
  s.name = 'ProtectedMedia'
  s.version = '1.0.0'
  s.summary = 'Authenticated local media transport for Kinosail playback'
  s.description = s.summary
  s.license = { :type => 'LicenseRef-Kinosail' }
  s.author = 'Kinosail'
  s.homepage = 'https://kinosail.com'
  s.source = { :git => 'https://github.com/MikeO7/kinosail.git' }
  s.platforms = { :ios => '16.4', :tvos => '16.4' }
  s.swift_version = '5.9'
  s.static_framework = true
  s.dependency 'ExpoModulesCore'
  s.dependency 'VLCKit', '4.0.0a24'
  s.source_files = '**/*.{swift,h,m}'
  s.public_header_files = 'KinosailAudioProcessor.h'
end
