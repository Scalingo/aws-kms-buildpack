package main

import (
	"archive/zip"
	"fmt"
	"io"
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

const maxZipEntrySizeBytes uint64 = 32 * 1024 * 1024

type downloadItem struct {
	dest  string
	obj   string
	entry string
}

type downloadPlan struct {
	plain map[string]string
	zips  map[string][]downloadItem
}

func main() {
	validateEnv()
	objects := strings.Split(os.Getenv("OBJECTS"), ",")
	files := strings.Split(os.Getenv("FILES"), ",")

	if len(objects) != len(files) {
		log.Println("FILES length is not the same as OBJECTS length")
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

	plan, err := buildDownloadPlan(objects, files, basePath)
	if err != nil {
		log.Println(err)
		os.Exit(-1)
	}

	downloader := s3manager.NewDownloader(awsConfig())

	err = executePlan(downloader, plan)
	if err != nil {
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
	for objectKey, dest := range plan.plain {
		err := downloadRawObject(downloader, dest, objectKey)
		if err != nil {
			return fmt.Errorf("failed to download object %q: %w", objectKey, err)
		}
	}

	for zipObject, items := range plan.zips {
		tmpPath, err := downloadZipTempFile(downloader, zipObject)
		if err != nil {
			return err
		}
		defer func(path string) {
			removeErr := os.Remove(path)
			if removeErr != nil && !os.IsNotExist(removeErr) {
				log.Printf("failed to remove temporary zip file %q: %v", path, removeErr)
			}
		}(tmpPath)

		err = extractZipEntries(tmpPath, items)
		if err != nil {
			return err
		}
	}

	return nil
}

func downloadRawObject(downloader *s3manager.Downloader, dest, object string) error {
	log.Println("Downloading", object, "to", dest)
	err := os.MkdirAll(filepath.Dir(dest), 0700)
	if err != nil {
		return fmt.Errorf("failed to create destination directory for %q: %w", dest, err)
	}

	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("failed to create file %q: %w", dest, err)
	}
	defer func() {
		closeErr := f.Close()
		if closeErr != nil {
			log.Printf("failed to close file %q: %v", dest, closeErr)
		}
	}()
	bucket := os.Getenv("KMSBP_AWS_BUCKET")

	_, err = downloader.Download(f, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &object,
	})
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}

	return nil
}

func downloadZipTempFile(downloader *s3manager.Downloader, zipObject string) (string, error) {
	tmp, err := os.CreateTemp("", "kmsbp-*.zip")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file for zip object %q: %w", zipObject, err)
	}
	defer func() {
		closeErr := tmp.Close()
		if closeErr != nil {
			log.Printf("failed to close temporary zip file: %v", closeErr)
		}
	}()
	tmpPath := tmp.Name()
	bucket := os.Getenv("KMSBP_AWS_BUCKET")
	var downloadErr error
	defer func() {
		if downloadErr != nil {
			_ = os.Remove(tmpPath)
		}
	}()

	_, downloadErr = downloader.Download(tmp, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &zipObject,
	})
	if downloadErr != nil {
		return "", fmt.Errorf("failed to download zip object %q: %w", zipObject, downloadErr)
	}

	return tmpPath, nil
}

func extractZipEntries(zipPath string, items []downloadItem) error {
	zipReader, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("failed to open zip file %q: %w", zipPath, err)
	}
	defer func() {
		closeErr := zipReader.Close()
		if closeErr != nil {
			log.Printf("failed to close zip reader for %q: %v", zipPath, closeErr)
		}
	}()

	for _, item := range items {
		found := false
		for _, file := range zipReader.File {
			if file.Name != item.entry {
				continue
			}
			found = true

			if file.FileInfo().IsDir() {
				return fmt.Errorf("zip entry %q is a directory", item.entry)
			}

			if file.UncompressedSize64 > maxZipEntrySizeBytes {
				return fmt.Errorf("zip entry %q exceeds max allowed size of %d bytes", item.entry, maxZipEntrySizeBytes)
			}

			in, err := file.Open()
			if err != nil {
				return fmt.Errorf("failed to open zip entry %q: %w", item.entry, err)
			}
			defer func() {
				closeErr := in.Close()
				if closeErr != nil {
					log.Printf("failed to close zip entry %q: %v", item.entry, closeErr)
				}
			}()

			err = os.MkdirAll(filepath.Dir(item.dest), 0700)
			if err != nil {
				return fmt.Errorf("failed to create destination directory for %q: %w", item.dest, err)
			}

			out, err := os.Create(item.dest)
			if err != nil {
				return fmt.Errorf("failed to create file %q: %w", item.dest, err)
			}
			defer func() {
				closeErr := out.Close()
				if closeErr != nil {
					log.Printf("failed to close file %q: %v", item.dest, closeErr)
				}
			}()

			_, copyErr := io.CopyN(out, in, int64(maxZipEntrySizeBytes))
			if copyErr != nil && copyErr != io.EOF {
				_ = os.Remove(item.dest)
				return fmt.Errorf("failed to write zip entry %q to %q: %w", item.entry, item.dest, copyErr)
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
