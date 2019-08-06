package main

import (
	"fmt"
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
	needed = []string{"KMSBP_AWS_BUCKET", "KMSBP_AWS_REGION", "KMSBP_AWS_ID", "KMSBP_AWS_TOKEN", "CERTS_INSTALL_PATH", "OBJECTS", "FILES"}
)

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
		err := download(downloader, filepath.Join(basePath, files[i]), object)
		if err != nil {
			log.Println("fail to download object", object)
			os.Exit(-1)
		}
	}
}

func download(downloader *s3manager.Downloader, dest string, object string) error {
	log.Println("Downloading", object, "to", dest)

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
