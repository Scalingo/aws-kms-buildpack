require 'aws-sdk'
require 'fileutils'

def download(s3, install_path, key)
  puts "Downloading #{key} at #{install_path}"
  resp = s3.get_object({
    bucket: ENV["AWS_BUCKET"],
    key: key,
    ssekms_key_id: ENV["KMS_KEY_ID"],
    response_target: install_path,
  })
end

needed = ["AWS_BUCKET", "AWS_REGION", "AWS_ID", "AWS_TOKEN", "CERTS_INSTALL_PATH", "OBJECTS", "FILES"]

needed.each do |key|
  if ! ENV.has_key?(key) then
    puts "Missing key: #{key}"
    exit 0
  end
end

objects = ENV["OBJECTS"].split(",")
files = ENV["FILES"].split(",")

if objects.length != files.length then
  puts "Files length is not the same as objects length"
  exit 1
end

base_path = "#{ENV["BUILD_DIR"]}/#{ENV["CERTS_INSTALL_PATH"]}"

FileUtils.mkdir_p base_path

Aws.config.update({
  region: ENV["AWS_REGION"],
  credentials: Aws::Credentials.new(ENV["AWS_ID"], ENV["AWS_TOKEN"])
})

s3 = Aws::S3::Client.new({region: ENV["AWS_REGION"]})

for i in 0..objets.length-1 do
  download(s3, "#{base_path}/#{files[i]}", objets[i])
end

