# Scalingo aws kms buildpack

This buildpack is used to download certificate from our S3 bucket.

To use it, you need to set the `BUILDPACK_URL` environment variable of your app to: `https://github.com/Scalingo/multi-buildpack.git`
and add the following line to your `.buildpacks` file:

```
https://github.com/Scalingo/aws-kms-buildpack.git
```

This script use the following environment variables:

* `AWS_BUCKET`: The name of the bucket
* `AWS_REGION`: The name of the region of the bucket (and the sse key)
* `AWS_ID`: The aws user id
* `AWS_TOKEN`: The aws user token
* `CERTS_INSTALL_PATH`: Path to the certificates
* `OBJECTS`: See below
* `FILES`: See below

The `OBJECTS` and `FILES` are to comma separated strings representing the objects to download from S3 and their filenames on the hardrive.

If we have the following configuration:

```
OBJECTS=a,b,c
FILES=1.txt,2.txt,3.txt
```

The buildpack will download the object a from s3 and store him in the `$CERTS_INSTALL_PATH/1.txt` file, store the b object to `$CERTS_INSTALL_PATH/2.txt` and store the c object to `$CERTS_INSTALL_PATH/3.txt`.
