Gem::Specification.new do |s|
    s.name        = "ocfl-index"
    s.version     = "0.0.0"
    s.summary     = "ruby client for ocfl-index"
    s.description = "ruby client for ocfl-index"
    s.authors     = ["Seth Erickson"]
    s.email       = "serickson@ucsb.edu"
    s.homepage    = "https://github.com/srerickson/ocfl-index"
    s.license     = "MIT"

    s.files        = Dir["{lib}/**/*.rb",]
    s.require_path = 'lib'

    s.add_dependency "grpc", "~> 1.50"
  end
  