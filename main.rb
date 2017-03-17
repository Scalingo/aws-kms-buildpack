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

needed = ["AWS_BUCKET", "AWS_REGION", "AWS_ID", "AWS_TOKEN", "CERTS_INSTALL_PATH", "CA_KEY", "CERT_KEY", "PRIVATE_KEY"]

needed.each do |key|
  if ! ENV.has_key?(key) then
    puts "Missing key: #{key}"
    exit 0
  end
end

base_path = "#{ENV["BUILD_DIR"]}/#{ENV["CERTS_INSTALL_PATH"]}"

FileUtils.mkdir_p base_path

Aws.config.update({
  region: ENV["AWS_REGION"],
  credentials: Aws::Credentials.new(ENV["AWS_ID"], ENV["AWS_TOKEN"])
})


s3 = Aws::S3::Client.new({region: ENV["AWS_REGION"]})

download(s3, "#{base_path}/ca.crt", ENV["CA_KEY"])
download(s3, "#{base_path}/cert.crt", ENV["CERT_KEY"])
download(s3, "#{base_path}/private.key", ENV["PRIVATE_KEY"])




