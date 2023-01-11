require 'gruf'
require 'ocfl-index'
Gruf.configure do |c|
  c.default_client_host = 'ocfl-index.fly.dev:443'
  c.default_channel_credentials = GRPC::Core::ChannelCredentials.new
end

begin
    client = ::Gruf::Client.new(service: OCFLIndex)
    resp = client.call(:GetSummary)
    puts resp.message.inspect
rescue Gruf::Client::Error => e
    puts e.error.inspect # If an error occurs, this will be the underlying error object
end