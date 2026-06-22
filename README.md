## Scalingo AWS KMS Buildpack

This buildpack is used to download certificates from a S3 bucket.

To use it, add the following line at the top of your `.buildpacks` file:

```
https://github.com/Scalingo/aws-kms-buildpack.git#v5.0.0
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

By default, each `OBJECTS` entry is treated as a plain S3 object key.
You can also extract a file from a zip object with this syntax:

```
<zip-object>:<path-inside-zip>
```

For example, `secrets/bundle.zip:tls/server.crt` means:
download `secrets/bundle.zip` from S3, then extract `tls/server.crt` from that zip and write it to the matching `FILES` path.

If we have the following configuration:

```
OBJECTS=a,b,c
FILES=1.txt,2.txt,3.txt
```

The buildpack will download the object `a` from S3 and store it in the `$CERTS_INSTALL_PATH/1.txt` file, store the `b` object to `$CERTS_INSTALL_PATH/2.txt` and store the `c` object to `$CERTS_INSTALL_PATH/3.txt`.

Mixed example with both plain objects and zip entries:

```
OBJECTS=certs/root.crt,secrets/bundle.zip:tls/server.crt
FILES=root.crt,server.crt
```


### Release

1. Place yourself on the commit you want to release

```bash
# x.y.z is the version number for this release
cd support/kmsbp/ && $SCALINGO_HOME/tools-and-hacks/prod-release/sc-release-odr-38 x.y.z
```

2. A pull request is created and all commit authors will be assigned as reviewers
3. Once the PR is merged, a "stable" tag is generated and a release out of it.
