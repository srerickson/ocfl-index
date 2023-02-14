# Generated by the protocol buffer compiler.  DO NOT EDIT!
# source: ocfl/v0/index.proto

require 'google/protobuf'

require 'google/protobuf/timestamp_pb'

Google::Protobuf::DescriptorPool.generated_pool.build do
  add_file("ocfl/v0/index.proto", :syntax => :proto3) do
    add_message "ocfl.v0.GetSummaryRequest" do
    end
    add_message "ocfl.v0.GetSummaryResponse" do
      optional :root_path, :string, 1, json_name: "rootPath"
      optional :spec, :string, 2, json_name: "spec"
      optional :description, :string, 3, json_name: "description"
      optional :num_objects, :int32, 4, json_name: "numObjects"
      optional :indexed_at, :message, 5, "google.protobuf.Timestamp", json_name: "indexedAt"
    end
    add_message "ocfl.v0.ListObjectsRequest" do
      optional :page_token, :string, 1, json_name: "pageToken"
      optional :page_size, :int32, 2, json_name: "pageSize"
      optional :order_by, :message, 3, "ocfl.v0.ListObjectsRequest.Sort", json_name: "orderBy"
    end
    add_message "ocfl.v0.ListObjectsRequest.Sort" do
      optional :field, :enum, 1, "ocfl.v0.ListObjectsRequest.Sort.Field", json_name: "field"
      optional :order, :enum, 2, "ocfl.v0.ListObjectsRequest.Sort.Order", json_name: "order"
    end
    add_enum "ocfl.v0.ListObjectsRequest.Sort.Field" do
      value :FIELD_UNSPECIFIED, 0
      value :FIELD_ID, 1
      value :FIELD_V1_CREATED, 2
      value :FIELD_HEAD_CREATED, 3
    end
    add_enum "ocfl.v0.ListObjectsRequest.Sort.Order" do
      value :ORDER_UNSPECIFIED, 0
      value :ORDER_ASC, 1
      value :ORDER_DESC, 2
    end
    add_message "ocfl.v0.ListObjectsResponse" do
      repeated :objects, :message, 1, "ocfl.v0.ListObjectsResponse.Object", json_name: "objects"
      optional :next_page_token, :string, 2, json_name: "nextPageToken"
    end
    add_message "ocfl.v0.ListObjectsResponse.Object" do
      optional :object_id, :string, 1, json_name: "objectId"
      optional :head, :string, 2, json_name: "head"
      optional :v1_created, :message, 3, "google.protobuf.Timestamp", json_name: "v1Created"
      optional :head_created, :message, 4, "google.protobuf.Timestamp", json_name: "headCreated"
    end
    add_message "ocfl.v0.GetObjectRequest" do
      optional :object_id, :string, 1, json_name: "objectId"
    end
    add_message "ocfl.v0.GetObjectResponse" do
      optional :object_id, :string, 1, json_name: "objectId"
      optional :spec, :string, 2, json_name: "spec"
      optional :root_path, :string, 3, json_name: "rootPath"
      optional :digest_algorithm, :string, 4, json_name: "digestAlgorithm"
      repeated :versions, :message, 5, "ocfl.v0.GetObjectResponse.Version", json_name: "versions"
    end
    add_message "ocfl.v0.GetObjectResponse.Version" do
      optional :num, :string, 1, json_name: "num"
      optional :message, :string, 2, json_name: "message"
      optional :created, :message, 3, "google.protobuf.Timestamp", json_name: "created"
      proto3_optional :user, :message, 4, "ocfl.v0.GetObjectResponse.Version.User", json_name: "user"
      optional :size, :int64, 5, json_name: "size"
      optional :has_size, :bool, 6, json_name: "hasSize"
    end
    add_message "ocfl.v0.GetObjectResponse.Version.User" do
      optional :name, :string, 1, json_name: "name"
      optional :address, :string, 2, json_name: "address"
    end
    add_message "ocfl.v0.GetObjectStateRequest" do
      optional :object_id, :string, 1, json_name: "objectId"
      optional :version, :string, 2, json_name: "version"
      optional :base_path, :string, 3, json_name: "basePath"
      optional :recursive, :bool, 4, json_name: "recursive"
      optional :page_token, :string, 5, json_name: "pageToken"
      optional :page_size, :int32, 6, json_name: "pageSize"
    end
    add_message "ocfl.v0.GetObjectStateResponse" do
      optional :digest, :string, 1, json_name: "digest"
      optional :isdir, :bool, 2, json_name: "isdir"
      optional :size, :int64, 3, json_name: "size"
      optional :has_size, :bool, 4, json_name: "hasSize"
      repeated :children, :message, 5, "ocfl.v0.GetObjectStateResponse.Item", json_name: "children"
      optional :next_page_token, :string, 6, json_name: "nextPageToken"
    end
    add_message "ocfl.v0.GetObjectStateResponse.Item" do
      optional :name, :string, 1, json_name: "name"
      optional :isdir, :bool, 2, json_name: "isdir"
      optional :size, :int64, 3, json_name: "size"
      optional :has_size, :bool, 4, json_name: "hasSize"
      optional :digest, :string, 5, json_name: "digest"
    end
    add_message "ocfl.v0.GetContentRequest" do
      optional :digest, :string, 1, json_name: "digest"
    end
    add_message "ocfl.v0.GetContentResponse" do
      optional :data, :bytes, 1, json_name: "data"
    end
  end
end

module Ocfl
  module V0
    GetSummaryRequest = ::Google::Protobuf::DescriptorPool.generated_pool.lookup("ocfl.v0.GetSummaryRequest").msgclass
    GetSummaryResponse = ::Google::Protobuf::DescriptorPool.generated_pool.lookup("ocfl.v0.GetSummaryResponse").msgclass
    ListObjectsRequest = ::Google::Protobuf::DescriptorPool.generated_pool.lookup("ocfl.v0.ListObjectsRequest").msgclass
    ListObjectsRequest::Sort = ::Google::Protobuf::DescriptorPool.generated_pool.lookup("ocfl.v0.ListObjectsRequest.Sort").msgclass
    ListObjectsRequest::Sort::Field = ::Google::Protobuf::DescriptorPool.generated_pool.lookup("ocfl.v0.ListObjectsRequest.Sort.Field").enummodule
    ListObjectsRequest::Sort::Order = ::Google::Protobuf::DescriptorPool.generated_pool.lookup("ocfl.v0.ListObjectsRequest.Sort.Order").enummodule
    ListObjectsResponse = ::Google::Protobuf::DescriptorPool.generated_pool.lookup("ocfl.v0.ListObjectsResponse").msgclass
    ListObjectsResponse::Object = ::Google::Protobuf::DescriptorPool.generated_pool.lookup("ocfl.v0.ListObjectsResponse.Object").msgclass
    GetObjectRequest = ::Google::Protobuf::DescriptorPool.generated_pool.lookup("ocfl.v0.GetObjectRequest").msgclass
    GetObjectResponse = ::Google::Protobuf::DescriptorPool.generated_pool.lookup("ocfl.v0.GetObjectResponse").msgclass
    GetObjectResponse::Version = ::Google::Protobuf::DescriptorPool.generated_pool.lookup("ocfl.v0.GetObjectResponse.Version").msgclass
    GetObjectResponse::Version::User = ::Google::Protobuf::DescriptorPool.generated_pool.lookup("ocfl.v0.GetObjectResponse.Version.User").msgclass
    GetObjectStateRequest = ::Google::Protobuf::DescriptorPool.generated_pool.lookup("ocfl.v0.GetObjectStateRequest").msgclass
    GetObjectStateResponse = ::Google::Protobuf::DescriptorPool.generated_pool.lookup("ocfl.v0.GetObjectStateResponse").msgclass
    GetObjectStateResponse::Item = ::Google::Protobuf::DescriptorPool.generated_pool.lookup("ocfl.v0.GetObjectStateResponse.Item").msgclass
    GetContentRequest = ::Google::Protobuf::DescriptorPool.generated_pool.lookup("ocfl.v0.GetContentRequest").msgclass
    GetContentResponse = ::Google::Protobuf::DescriptorPool.generated_pool.lookup("ocfl.v0.GetContentResponse").msgclass
  end
end
