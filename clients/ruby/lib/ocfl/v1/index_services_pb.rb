# Generated by the protocol buffer compiler.  DO NOT EDIT!
# Source: ocfl/v1/index.proto for package 'ocfl.v1'

require 'grpc'
require 'ocfl/v1/index_pb'

module Ocfl
  module V1
    module IndexService
      # IndexService is used to index and query OCFL objects in a repository.
      class Service

        include ::GRPC::GenericService

        self.marshal_class_method = :encode
        self.unmarshal_class_method = :decode
        self.service_name = 'ocfl.v1.IndexService'

        # Get index status, counts, and storage root details
        rpc :GetStatus, ::Ocfl::V1::GetStatusRequest, ::Ocfl::V1::GetStatusResponse
        # Start an asynchronous indexing process to scan the storage root and ingest
        # index inventories. Indexed objects not found during the storage root scan
        # are removed from the index. IndexAll returns immediately with a status
        # indicating whether the indexing process was started.
        rpc :IndexAll, ::Ocfl::V1::IndexAllRequest, ::Ocfl::V1::IndexAllResponse
        # Index inventories for the specified object ids. Unlike IndexAll, IndexIDs
        # returns after the object ids have been indexed.
        rpc :IndexIDs, ::Ocfl::V1::IndexIDsRequest, ::Ocfl::V1::IndexIDsResponse
        # List all objects in the index in lexigraphical order by ID.
        rpc :ListObjects, ::Ocfl::V1::ListObjectsRequest, ::Ocfl::V1::ListObjectsResponse
        # Get details for a specific object in the index
        rpc :GetObject, ::Ocfl::V1::GetObjectRequest, ::Ocfl::V1::GetObjectResponse
        # Query the logical state of an OCFL object version
        rpc :GetObjectState, ::Ocfl::V1::GetObjectStateRequest, ::Ocfl::V1::GetObjectStateResponse
        # Stream log messages from indexing tasks
        rpc :FollowLogs, ::Ocfl::V1::FollowLogsRequest, stream(::Ocfl::V1::FollowLogsResponse)
      end

      Stub = Service.rpc_stub_class
    end
  end
end
