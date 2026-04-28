package main

import (
	"archive/zip"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/defaults"
	"github.com/aws/aws-sdk-go-v2/aws/endpoints"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/s3manager"
)

var (
	needed = []string{"KMSBP_AWS_BUCKET", "KMSBP_AWS_REGION", "KMSBP_AWS_ID", "KMSBP_AWS_TOKEN", "CERTS_INSTALL_PATH", "OBJECTS", "FILES", "BUILD_DIR"}
)

const zipSpecSeparator = ":"

func main() {
	validateEnv()
	objects := strings.Split(os.Getenv("OBJECTS"), ",")
	files := strings.Split(os.Getenv("FILES"), ",")

	if len(objects) != len(files) {
		log.Println("FILES length is not the same as OBEJCTS length")
		os.Exit(-1)
	}

	basePath := fmt.Sprintf(
		"%s%s", os.Getenv("BUILD_DIR"), os.Getenv("CERTS_INSTALL_PATH"),
	)

	err := os.MkdirAll(basePath, 0700)
	if err != nil {
		log.Println("fail to make base path:", err)
		os.Exit(-1)
	}

	downloader := s3manager.NewDownloader(awsConfig())
	for i, object := range objects {
		object = strings.TrimSpace(object)
		file := strings.TrimSpace(files[i])

		objectKey, zipEntry, isZipEntry, err := parseObjectSpec(object)
		if err != nil {
			log.Println(err)
			os.Exit(-1)
		}

		dest := filepath.Join(basePath, file)
		if isZipEntry {
			err = downloadFromZipObject(downloader, dest, objectKey, zipEntry)
		} else {
			err = download(downloader, dest, objectKey)
		}
		if err != nil {
			log.Println("fail to download object", object)
			os.Exit(-1)
		}
	}
}

func parseObjectSpec(spec string) (string, string, bool, error) {
	parts := strings.SplitN(spec, zipSpecSeparator, 2)
	if len(parts) == 1 {
		return spec, "", false, nil
	}

	object := strings.TrimSpace(parts[0])
	entry := strings.TrimSpace(parts[1])
	if object == "" || entry == "" {
		return "", "", false, fmt.Errorf("invalid object spec %q (expected <object> or <zip-object>%s<entry>)", spec, zipSpecSeparator)
	}

	return object, entry, true, nil
}

func download(downloader *s3manager.Downloader, dest string, object string) error {
	log.Println("Downloading", object, "to", dest)
	if err := os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
		return fmt.Errorf("failed to create destination directory for %q, %v", dest, err)
	}

	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("failed to create file %q, %v", dest, err)
	}
	defer f.Close()

	_, err = downloader.Download(f, &s3.GetObjectInput{
		Bucket: aws.String(os.Getenv("KMSBP_AWS_BUCKET")),
		Key:    aws.String(object),
	})
	if err != nil {
		return fmt.Errorf("failed to upload file, %v", err)
	}
	return nil
}

func downloadFromZipObject(downloader *s3manager.Downloader, dest string, object string, zipEntry string) error {
	log.Println("Downloading", zipEntry, "from", object, "to", dest)

	tmp, err := ioutil.TempFile("", "kmsbp-*.zip")
	if err != nil {
		return fmt.Errorf("failed to create temporary file for zip object %q, %v", object, err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	_, err = downloader.Download(tmp, &s3.GetObjectInput{
		Bucket: aws.String(os.Getenv("KMSBP_AWS_BUCKET")),
		Key:    aws.String(object),
	})
	if closeErr := tmp.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("failed to download zip object %q, %v", object, err)
	}

	zr, err := zip.OpenReader(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to open zip object %q, %v", object, err)
	}
	defer zr.Close()

	for _, file := range zr.File {
		if file.Name != zipEntry {
			continue
		}
		if file.FileInfo().IsDir() {
			return fmt.Errorf("zip entry %q from object %q is a directory", zipEntry, object)
		}

		in, err := file.Open()
		if err != nil {
			return fmt.Errorf("failed to open zip entry %q from object %q, %v", zipEntry, object, err)
		}
		defer in.Close()

		if err := os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
			return fmt.Errorf("failed to create destination directory for %q, %v", dest, err)
		}

		out, err := os.Create(dest)
		if err != nil {
			return fmt.Errorf("failed to create file %q, %v", dest, err)
		}

		_, copyErr := io.Copy(out, in)
		closeErr := out.Close()
		if copyErr != nil {
			return fmt.Errorf("failed to write zip entry %q to %q, %v", zipEntry, dest, copyErr)
		}
		if closeErr != nil {
			return fmt.Errorf("failed to close file %q, %v", dest, closeErr)
		}

		return nil
	}

	return fmt.Errorf("zip entry %q not found in object %q", zipEntry, object)
}

func awsConfig() aws.Config {
	creds := aws.NewStaticCredentialsProvider(
		os.Getenv("KMSBP_AWS_ID"), os.Getenv("KMSBP_AWS_TOKEN"), "",
	)
	c := aws.Config{
		Region:           os.Getenv("KMSBP_AWS_REGION"),
		Credentials:      creds,
		Handlers:         defaults.Handlers(),
		HTTPClient:       defaults.HTTPClient(),
		EndpointResolver: endpoints.NewDefaultResolver(),
	}
	if os.Getenv("KMSBP_AWS_ENDPOINT") != "" {
		c.EndpointResolver = aws.ResolveWithEndpoint(aws.Endpoint{
			URL:           "https://" + os.Getenv("KMSBP_AWS_ENDPOINT"),
			SigningRegion: os.Getenv("KMSBP_AWS_REGION"),
		})
	}
	return c
}

func validateEnv() {
	for _, name := range needed {
		if os.Getenv(name) == "" {
			log.Println("Missing environment variable", name)
			os.Exit(-1)
		}
	}
}
