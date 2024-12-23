## Scalingo AWS KMS Buildpack

This buildpack is used to download certificates from a S3 bucket.

To use it, add the following line at the top of your `.buildpacks` file:

```
https://github.com/Scalingo/aws-kms-buildpack.git#v4
```

This buildpack uses the following environment variables:

* `KMSBP_AWS_BUCKET`: The name of the bucket
* `KMSBP_AWS_REGION`: The name of the region of the bucket (and the sse key)
* `KMSBP_AWS_ID`: The AWS user ID
* `KMSBP_AWS_TOKEN`: The AWS user token
* `CERTS_INSTALL_PATH`: Path to the certificates
* `OBJECTS`: See below
* `FILES`: See below

The `OBJECTS` and `FILES` are two comma separated strings representing the objects to download from S3 and their filenames on the hardrive.

If we have the following configuration:

```
OBJECTS=a,b,c
FILES=1.txt,2.txt,3.txt
```

The buildpack will download the object `a` from S3 and store it in the `$CERTS_INSTALL_PATH/1.txt` file, store the `b` object to `$CERTS_INSTALL_PATH/2.txt` and store the `c` object to `$CERTS_INSTALL_PATH/3.txt`.
