# Generated by the protocol buffer compiler.  DO NOT EDIT!
# Source: ocfl/v0/index.proto for package 'ocfl.v0'

require 'grpc'
require 'ocfl/v0/index_pb'

module Ocfl
  module V0
    module IndexService
      class Service

        include ::GRPC::GenericService

        self.marshal_class_method = :encode
        self.unmarshal_class_method = :decode
        self.service_name = 'ocfl.v0.IndexService'

        # Basic info on the storage root & index status.
        rpc :GetSummary, ::Ocfl::V0::GetSummaryRequest, ::Ocfl::V0::GetSummaryResponse
        # OCFL Objects in the index
        rpc :ListObjects, ::Ocfl::V0::ListObjectsRequest, ::Ocfl::V0::ListObjectsResponse
        # Details on a specific OCFL object in the index 
        rpc :GetObject, ::Ocfl::V0::GetObjectRequest, ::Ocfl::V0::GetObjectResponse
        # Query the logical state of an OCFL object version
        rpc :GetObjectState, ::Ocfl::V0::GetObjectStateRequest, ::Ocfl::V0::GetObjectStateResponse
        # Download byte stream for content (based on digest)
        rpc :GetContent, ::Ocfl::V0::GetContentRequest, stream(::Ocfl::V0::GetContentResponse)
      end

      Stub = Service.rpc_stub_class
    end
  end
end