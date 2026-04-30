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

type downloadItem struct {
	dest  string
	obj   string
	entry string
}

type downloadPlan struct {
	plain map[string]string         // object -> dest
	zips  map[string][]downloadItem // zipObject -> []items
}

func main() {
	validateEnv()
	objects := strings.Split(os.Getenv("OBJECTS"), ",")
	files := strings.Split(os.Getenv("FILES"), ",")

	if len(objects) != len(files) {
		log.Println("FILES length is not the same as OBEJCTS length ")
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

	// Parse and build execution plan
	plan, err := buildDownloadPlan(objects, files, basePath)
	if err != nil {
		log.Println(err)
		os.Exit(-1)
	}

	// Execute plan
	downloader := s3manager.NewDownloader(awsConfig())

	if err := executePlan(downloader, plan); err != nil {
		log.Println(err)
		os.Exit(-1)
	}
}

func buildDownloadPlan(objectSpecs, fileDestinations []string, basePath string) (*downloadPlan, error) {
	plan := &downloadPlan{
		plain: make(map[string]string),
		zips:  make(map[string][]downloadItem),
	}

	for i, spec := range objectSpecs {
		spec = strings.TrimSpace(spec)
		dest := strings.TrimSpace(fileDestinations[i])
		fullDest := filepath.Join(basePath, dest)

		objectKey, zipEntry, isZipEntry, err := parseObjectSpec(spec)
		if err != nil {
			return nil, err
		}

		if isZipEntry {
			plan.zips[objectKey] = append(plan.zips[objectKey], downloadItem{
				dest:  fullDest,
				obj:   objectKey,
				entry: zipEntry,
			})
		} else {
			if existing, ok := plan.plain[objectKey]; ok && existing != fullDest {
				return nil, fmt.Errorf("object %q mapped to different destinations: %q and %q", objectKey, existing, fullDest)
			}
			plan.plain[objectKey] = fullDest
		}
	}

	return plan, nil
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

func executePlan(downloader *s3manager.Downloader, plan *downloadPlan) error {
	// Download plain objects
	for objectKey, dest := range plan.plain {
		if err := downloadRawObject(downloader, dest, objectKey); err != nil {
			return fmt.Errorf("failed to download object %q: %w", objectKey, err)
		}
	}

	// Download and extract zip objects
	for zipObject, items := range plan.zips {
		tmpPath, err := downloadZipTempFile(downloader, zipObject)
		if err != nil {
			return err
		}
		defer os.Remove(tmpPath)

		if err := extractZipEntries(tmpPath, items); err != nil {
			return err
		}
	}

	return nil
}

func downloadRawObject(downloader *s3manager.Downloader, dest, object string) error {
	log.Println("Downloading", object, "to", dest)
	if err := os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
		return fmt.Errorf("failed to create destination directory for %q: %w", dest, err)
	}

	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("failed to create file %q: %w", dest, err)
	}
	defer f.Close()

	_, err = downloader.Download(f, &s3.GetObjectInput{
		Bucket: aws.String(os.Getenv("KMSBP_AWS_BUCKET")),
		Key:    aws.String(object),
	})
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	return nil
}

func downloadZipTempFile(downloader *s3manager.Downloader, zipObject string) (string, error) {
	tmp, err := ioutil.TempFile("", "kmsbp-*.zip")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file for zip object %q: %w", zipObject, err)
	}
	tmpPath := tmp.Name()

	_, err = downloader.Download(tmp, &s3.GetObjectInput{
		Bucket: aws.String(os.Getenv("KMSBP_AWS_BUCKET")),
		Key:    aws.String(zipObject),
	})
	if closeErr := tmp.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("failed to download zip object %q: %w", zipObject, err)
	}

	return tmpPath, nil
}

func extractZipEntries(zipPath string, items []downloadItem) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("failed to open zip file %q: %w", zipPath, err)
	}
	defer zr.Close()

	for _, item := range items {
		found := false
		for _, file := range zr.File {
			if file.Name != item.entry {
				continue
			}
			found = true

			if file.FileInfo().IsDir() {
				return fmt.Errorf("zip entry %q is a directory", item.entry)
			}

			in, err := file.Open()
			if err != nil {
				return fmt.Errorf("failed to open zip entry %q: %w", item.entry, err)
			}

			if err := os.MkdirAll(filepath.Dir(item.dest), 0700); err != nil {
				_ = in.Close()
				return fmt.Errorf("failed to create destination directory for %q: %w", item.dest, err)
			}

			out, err := os.Create(item.dest)
			if err != nil {
				_ = in.Close()
				return fmt.Errorf("failed to create file %q: %w", item.dest, err)
			}

			_, copyErr := io.Copy(out, in)
			inCloseErr := in.Close()
			outCloseErr := out.Close()

			if copyErr != nil {
				return fmt.Errorf("failed to write zip entry %q to %q: %w", item.entry, item.dest, copyErr)
			}
			if inCloseErr != nil {
				return fmt.Errorf("failed to close zip entry %q: %w", item.entry, inCloseErr)
			}
			if outCloseErr != nil {
				return fmt.Errorf("failed to close file %q: %w", item.dest, outCloseErr)
			}

			log.Println("Downloaded", item.entry, "from", item.obj, "to", item.dest)
			break
		}

		if !found {
			return fmt.Errorf("zip entry %q not found in %q", item.entry, zipPath)
		}
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
