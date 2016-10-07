package torrent

import (
	"crypto/sha1"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"runtime"
	"sync"
	"time"

	"github.com/golang/glog"
	"repo.hovitos.engineering/mdye/torrent"
)

const HORIZON_WEBSEED_DEFAULT = "https://images.bluehorizon.network/"

func imageCheck(torrentDir string, images []string, fn func(string, *os.File) (bool, error)) (bool, error) {
	for _, imageFile := range images {
		if file, err := os.Open(path.Join(torrentDir, imageFile)); err != nil {
			return false, err
		} else {
			defer file.Close()
			_, strippedPath := path.Split(imageFile)

			if check, err := fn(strippedPath, file); err != nil {
				return false, err
			} else if !check {
				return false, nil
			}
		}
	}

	// all files checked out
	return true, nil
}

func CheckHashes(torrentDir string, images []string, imageHashes map[string]string) (bool, error) {

	sha1Check := func(filename string, file *os.File) (bool, error) {
		hasher := sha1.New()

		if _, err := io.Copy(hasher, file); err != nil {
			return false, err
		} else {
			hash := fmt.Sprintf("%x", string(hasher.Sum(nil)))
			expectedHash := imageHashes[filename]
			if expectedHash == "" {
				return false, fmt.Errorf("Expected hash (key: %v) not present in provided imageHashes: %v", filename, imageHashes)
			} else {
				return hash == imageHashes[filename], nil
			}
		}
	}

	return imageCheck(torrentDir, images, sha1Check)
}

func CheckSignatures(torrentDir string, images []string, imageSignatures map[string]string, publicKeyFile string) (bool, error) {

	signatureCheck := func(filename string, file *os.File) (bool, error) {
		expectedSignature := imageSignatures[filename]
		if expectedSignature == "" {
			return false, fmt.Errorf("Expected signature (key: %v) not present in provided imageSignatures: %v", filename, imageSignatures)
		} else {
			return verify(publicKeyFile, expectedSignature, file)
		}
	}

	return imageCheck(torrentDir, images, signatureCheck)
}

type FetchSignal struct {
	Error error // nil means complete
	File  string
}

func httpDl(httpClient *http.Client, torrentPath string, url string, group *sync.WaitGroup) {
	// download from web

	dirPath, filename := path.Split(torrentPath)

	if _, err := os.Stat(torrentPath); err == nil {
		glog.Infof("File exists on disk: %v, skipping.", torrentPath)

	} else if err := os.MkdirAll(dirPath, 0755); err != nil {
		glog.Errorf("Unable to create path to new file for torrent image in directory: %v. Error: %v", dirPath, err)
	} else if tmpImageFile, err := os.Create(fmt.Sprintf("%v-tmp", torrentPath)); err != nil {
		glog.Errorf("Unable to create new file for torrent image: %v in directory: %v. Error: %v", filename, dirPath, err)
	} else {
		defer tmpImageFile.Close()

		glog.V(3).Infof("Downloading torrentPath: %v from url: %v", torrentPath, url)
		response, err := httpClient.Get(url)

		if err != nil || response.StatusCode != 200 {
			glog.Errorf("HTTP request failed: %v, response: %v", err, response)
		} else {
			defer response.Body.Close()

			// copy Body content to file
			bytes, err := io.Copy(tmpImageFile, response.Body)
			if err != nil {
				glog.Errorf("iocopy from HTTP response body failed on file: %v. Error: %v", tmpImageFile, err)
			} else if err := os.Rename(tmpImageFile.Name(), torrentPath); err != nil {
				glog.Errorf("Failed to rename temporary image download file (%v) to final one: %v. Error: %v", tmpImageFile.Name(), torrentPath, err)

			} else {
				glog.Infof("Wrote %d image file bytes to %s\n", bytes, torrentPath)
			}
		}
	}

	group.Done()
}

// an alternative for resource-constrained clients, only uses webseed
func downloadTorrentFromWebSeed(files []torrent.File, seedURLs interface{}, httpClient *http.Client, torrentDir string) error {

	var webSeed string
	if seedURLs == nil {
		// try our default
		webSeed = HORIZON_WEBSEED_DEFAULT
	} else {
		seeds := seedURLs.([]string)
		// TODO: expand to try multiple webseeds
		webSeed = seeds[0]
	}

	var group sync.WaitGroup

	for _, file := range files {
		filename := file.Path()
		torrentPath := path.Join(torrentDir, filename)
		urlString := fmt.Sprintf("%v/%v", webSeed, filename)

		if url, err := url.Parse(urlString); err != nil {
			return fmt.Errorf("Unable to parse generated url string for web seed: %v. Error: %v", urlString, err)
		} else {
			group.Add(1)
			go httpDl(httpClient, torrentPath, url.String(), &group)
		}
	}

	group.Wait()

	glog.Infof("All files have been downloaded: %v", files)
	return nil
}

func downloadTorrent(torrentFile *os.File, client *torrent.Client, httpClient *http.Client, torrentDir string) ([]string, error) {

	torrent, err := client.AddTorrentFromFile(torrentFile.Name())
	if err != nil {
		return nil, err
	} else {
		<-torrent.GotInfo() // block until info is fetched

		// expecting at least one image file in torrent; error if this is not true
		if fileCt := len(torrent.Files()); fileCt == 0 {
			return nil, fmt.Errorf("Unexpected torrent file content, expecting at least one image file, instead found none. Torrent info: %v\n", torrent.Info())
		} else {
			glog.Infof("Torrent detail:\n  files: %v\ninfohash: %v\n", torrent.Files(), torrent.InfoHash())
			if runtime.GOARCH == "arm" || runtime.GOARCH == "arm64" || runtime.GOARCH == "amd64" || runtime.GOARCH == "386" {
				glog.Infof("Platform: %v. Using web seed for this platform to avoid taxing its meager resources.", runtime.GOARCH)
				if err := downloadTorrentFromWebSeed(torrent.Files(), torrent.MetaInfo().URLList, httpClient, torrentDir); err != nil {
					return nil, fmt.Errorf("Failed downloading torrent files from web seed. Error: %v", err)
				}
			} else {
				glog.Infof("Platform: %v. Using aggressive torrent client for this platform", runtime.GOARCH)
				torrent.DownloadAll()
				// TODO: this may cause client to wait too long in some circumstances: it blocks on *all* torrent downloads; should improve this by waiting only on a particular torrent to finish downloading
				client.WaitAll()
			}

			imageFiles := make([]string, fileCt)

			for ix, file := range torrent.Files() {
				imageFiles[ix] = file.Path()
			}

			glog.V(2).Infof("Succeeded downloading imageFiles via torrent: %v", imageFiles)
			return imageFiles, nil
		}
	}
}

func httpClient(caFile string) (*http.Client, error) {
	caBytes, err := ioutil.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("Failed to read CACertsFile: %v", caFile)
	}

	var tls tls.Config
	tls.InsecureSkipVerify = false

	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(caBytes)
	tls.RootCAs = certPool

	tls.BuildNameToCertificate()

	return &http.Client{
		Timeout: time.Second * 360,
		Transport: &http.Transport{
			TLSClientConfig: &tls,
		},
	}, nil
}

// Fetch Uses given torrent client to fetch torrent w/ torrent file at given url to the destination configured with the Client. Checks given SHA1 hash of the downloaded image file or errors. Blocks until content is fetched.
func Fetch(url url.URL, imageHashes map[string]string, imageSignatures map[string]string, caCertsPath string, torrentDir string, publicKeyFile string, client *torrent.Client) ([]string, error) {

	httpClient, err := httpClient(caCertsPath)
	if err != nil {
		return []string{}, err
	}

	torrentFile, err := ioutil.TempFile("", "anax_torrent") // creates tmpfile in os.TempDir
	if err != nil {
		return []string{}, fmt.Errorf("Unable to create temporary directory client: %s", err)
	} else {
		defer os.Remove(torrentFile.Name())
		response, err := httpClient.Get(url.String())
		responseError := fmt.Errorf("Unable to fetch torrent file at %s: %s. Response: %v", url.String(), err, response)

		if err != nil || response.StatusCode != 200 {
			return nil, responseError
		} else {
			defer response.Body.Close()

			// copy Body content to file
			bytes, err := io.Copy(torrentFile, response.Body)
			if err != nil {
				return nil, responseError
			} else {
				glog.Infof("Wrote %d torrent file bytes to %s\n", bytes, torrentFile.Name())

				for _, torrent := range client.Torrents() {
					glog.Infof("Torrent client already knows about torrent with infohash: %v\n", torrent.InfoHash())
				}

				glog.Infof("Downloading torrent file %v", url.String())

				images, err := downloadTorrent(torrentFile, client, httpClient, torrentDir)
				if err != nil {
					return nil, fmt.Errorf("Unable to fetch image file(s) using torrent file: %s. Error: %s", torrentFile.Name(), err)
				} else {

					if hashCheck, err := CheckHashes(torrentDir, images, imageHashes); err != nil {
						return nil, fmt.Errorf("Error checking hashes provided for images: %v. Error: %v", imageHashes, err)
					} else if signatureCheck, err := CheckSignatures(torrentDir, images, imageSignatures, publicKeyFile); err != nil {
						return nil, fmt.Errorf("Error checking cryptographic signatures provided for images: %v. Error: %v", imageSignatures, err)
					} else if !(hashCheck && signatureCheck) {
						return nil, errors.New("Invalid hashes or signatures for images, refusing to load")
					} else {
						// success!
						glog.Infof("Finished downloading image content: %v", images)

						// important to make sure each

						return images, nil
					}
				}
			}
		}
	}
}
