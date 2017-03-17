require 'aws-sdk'
require 'fileutils'

def download(s3, install_path, key)
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

FileUtils.mkdir_p ENV["CERTS_INSTALL_PATH"]

Aws.config.update({
  region: ENV["AWS_REGION"],
  credentials: Aws::Credentials.new(ENV["AWS_ID"], ENV["AWS_TOKEN"])
})


s3 = Aws::S3::Client.new({region: ENV["AWS_REGION"]})

download(s3, "#{ENV["CERTS_INSTALL_PATH"]}/ca.crt", ENV["CA_KEY"])
download(s3, "#{ENV["CERTS_INSTALL_PATH"]}/cert.crt", ENV["CERT_KEY"])
download(s3, "#{ENV["CERTS_INSTALL_PATH"]}/private.key", ENV["PRIVATE_KEY"])




