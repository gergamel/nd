# iNDelible

Minimum viable permanent storage and retrieval over HTTP.

## Background

Forked from [github.com/git-lfs/lfs-test-server](https://github.com/git-lfs/lfs-test-server), so all credit to the [these people](https://github.com/git-lfs/lfs-test-server/graphs/contributors) for doing most of the work.

The iNDelible server is intended to provide a minimal, HTTP object store with the following specific contraints:
- Will accept and store any multipart file via HTTP PUT, responding with a unique object ID (initially the SHA256 of the file, but this will probably change) which can then be used to later retrieve the file and/or its metadata.
- Stored objects are permanent and immutable (hence the name).
- Must be able to handle large files (>4GB).

ND will implement the storage component of a larger service geared towards Product Information Management (PIM). A fair bit of Googling was done before starting this, but it's as much an opportunity to finally learn golang properly as to end up with a minimal viable storage layer. At the time of writing, there are two projects that look they would do well out of the box:
- [Perkeep](https://perkeep.org/) (nee Camlistore) which is almost exactly what I want, especially the rsync-style deduping using rolling hash. Appears to use a [Merkle Tree](https://en.wikipedia.org/wiki/Merkle_tree) for the object store?
- [OpenStack Swift](https://wiki.openstack.org/wiki/Swift) (TODO: morrisjobke/docker-swift-onlyone) which wouldn't just drop in easily, but might be a smart choice because it is designed to scale massively.

## Details

Implementation-wise, the first cut has the following:
* Binary storage is pure filesystem, but the plan is to learn more about chunking/deduping as done in rsync, Perkeep, Spideroak etc...
* Metadata storage uses a [Bolt](https://github.com/boltdb/bolt) key/value DB. Currently we store the FileName (from the client), ContentType, Length (bytes) and the creation date (as a Unix timestamp).
* Content-Type is inferred from the stream upon storage (using net/http/DetectContentType) because it's way more reliable than listening to what the client thinks.
* [http://localhost:8080/objects]() will give you a JSON list of hashes
* [http://localhost:8080/meta/{hash}]() will give you a JSON string of the object's metadata
* [http://localhost:8080/objects/{hash}]() will return the object itself as a Content-Disposition inline ... so that the file will be rendered by the browser if possible (user friendly for PDFs etc...)

## TODO

* Restore lfs-test-server code which used the request's accepts header to allow a single URL to return either the file or its metadata
* Chunking/deduping binary storage
* Authentication, not actually a huge priority because no edits or deletions are allowed, but still...

## Golang setup
* Run this:
  ```bash
  VERSION=1.10.3
  OS=linux
  ARCH=amd64
  
  wget https://dl.google.com/go/go$VERSION.$OS-$ARCH.tar.gz
  tar -C /usr/local -xzf go$VERSION.$OS-$ARCH.tar.gz
  
  echo '
  # Added by golang install
  export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
  ```
* Then this, to check it's working:
  ```bash
  go --version
  ```

## Data store
The default data store is /var/opt/indelible, so you might want to:
```bash
sudo mkdir /var/opt/indelible
sudo chown <user>:<user> /var/opt/indelible
```

Alternatively, you can override with ENV variables (see config.go).

# Building
Once you have go installed, build the binary and run it:
```bash
go build
```
