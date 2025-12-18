[![Build](https://img.shields.io/github/actions/workflow/status/OpenAds-Auction/OpenAdsServer/ci.yml?branch=main&style=flat-square)](https://github.com/OpenAds-Auction/OpenAdsServer/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/OpenAds-Auction/OpenAdsServer?style=flat-square)](https://goreportcard.com/report/github.com/OpenAds-Auction/OpenAdsServer)
![Go Version](https://img.shields.io/github/go-mod/go-version/OpenAds-Auction/OpenAdsServer?style=flat-square)


```
                                                                            
                                                                  ,,          
  .g8""8q.                                          db          `7MM          
.dP'    `YM.                                       ;MM:           MM          
dM'      `MM `7MMpdMAo.  .gP"Ya `7MMpMMMb.        ,V^MM.     ,M""bMM  ,pP"Ybd 
MM        MM   MM   `Wb ,M'   Yb  MM    MM       ,M  `MM   ,AP    MM  8I   `" 
MM.      ,MP   MM    M8 8M""""""  MM    MM       AbmmmqMA  8MI    MM  `YMMMa. 
`Mb.    ,dP'   MM   ,AP YM.    ,  MM    MM      A'     VML `Mb    MM  L.   I8 
  `"bmmd"'     MMbmmd'   `Mbmmd'.JMML  JMML.  .AMA.   .AMMA.`Wbmd"MML.M9mmmP' 
               MM                                                           
             .JMML.                                                                            
```

# Overview

[OpenAds](https://openads.com) is a high-integrity auction environment, powered by Prebid-derived open source auction logic.

The OpenAds Server is the heart of the OpenAds auction environment. It is responsible for receiving bid requests from OpenAds thin clients, processing them, and returning bid responses.


## Attestation

OpenAds Server builds are designed to be determistic, reproducible, and auditable.

### Determinism and Reproducibility
Determinism is better achieved through the following changes when compared to Prebid Server:

- Added a `.dockerignore` file to exclude unnecessary files from the build. 
- Pinned dependencies (e.g. Ubuntu base image) in Dockerfile.
- Pinned Golang version which is confirmed with a checksum.
- Downloaded modules are compare against `go.sum`, then build image with trimpath and PIE flags.

As Docker builds will contain dynamic cryptographic data (more on that below), reproducibility can be demonstrated by building the binary directly:

```aiignore
    go build \
        -mod=vendor \
        -trimpath \
        -buildmode=pie \
        -o openads .
```

And then comparing hashes of the binary across builds. 

### Auditability

OpenAds Server builds are created with a Github Actions workflow. That workflow creates a one-time RSA keypair which is used to sign the build. The private key is destroyed during the workflow process, but the public key is stored and available to check the build's integrity against the server's `/attestation` endpoint to ensure that the code built in this repository matches the code running on the server. In the future, the attestation process will be further strengthened through the use of secure compute environments, such as [AWS Nitro Enclaves](https://aws.amazon.com/ec2/nitro/nitro-enclaves/).

For more information about the OpenAds Server's attestation process and to conduct your own audits, please see the [attestation guide](docs/BUILD_ATTESTATION.md).
  
## Configuring

When developing locally, **you must set a `PBS_GDPR_DEFAULT_VALUE`**. This configuration determines whether GDPR is enabled when no regulatory signal is available in the request, where a value of `"0"` disables it by default and a value of `"1"` enables it. This is required as there is no consensus on a good default.

Refer to the [configuration guide](docs/developers/configuration.md) for additional information and a list of available configuration options.

## Developing

OpenAds Server requires [Go](https://go.dev) version 1.24 or newer. You can develop on any operating system that Go supports; however, please note that our helper scripts are written in bash.

1. Clone The Repository
``` bash
git clone git@github.com:OpenAds-Auction/OpenAdsServer.git
cd OpenAdsServer
```

3. Download Dependencies
``` bash
go mod download
```

3. Verify Tests Pass
```bash
./validate.sh
```

4. Run The Server
```bash
go run .
```

By default, OpenAds Server will attach to port 8000. To confirm the server is running, visit `http://localhost:8000/` in your web browser.

### Code Style
To maintain consistency in the project's code, please:
 
- Follow the recommendations set by [Effective Go](https://go.dev/doc/effective_go). This article provides a comprehensive guide on how to write idiomatic Go code, covering topics such as naming and formatting. Many IDEs will automatically format your code upon save. If you need to manaully format your code, either run the bash script or execute the make step:
   ```
   ./scripts/format.sh -f true
   ```
   ```
   make format
   ```

- Prefer small functions with descriptive names instead of complex functions with comments. This approach helps make the code more readable, maintainable, and testable.

- Do not discard errors. You should implement appropriate error handling, such as gracefully falling back to a default behavior or bubbling up an error.

## Contributing

We welcome contributions to OpenAds Server in form of issues and PRs in this repository. If your change applies to both Prebid Server as well as OpenAds Server, please submit it to the [Prebid Server Go repository](https://github.com/prebid/prebid-server) as we will pull in relevant upstream changes. 

### Acknowledgements

OpenAds is in part derived from <a href="https://github.com/prebid/prebid-server" target="_blank">Prebid Server.</a>