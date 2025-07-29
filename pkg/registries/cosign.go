package registries

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime"

	"github.com/rancher/charts-build-scripts/pkg/filesystem"
	"github.com/rancher/charts-build-scripts/pkg/logger"
	"github.com/rancher/charts-build-scripts/pkg/path"
	"sigs.k8s.io/release-utils/version"

	// go-containerregistry
	authn "github.com/google/go-containerregistry/pkg/authn"
	name "github.com/google/go-containerregistry/pkg/name"
	remote "github.com/google/go-containerregistry/pkg/v1/remote"
	transport "github.com/google/go-containerregistry/pkg/v1/remote/transport"

	// cosign
	oci "github.com/sigstore/cosign/v2/pkg/oci"
	ociremote "github.com/sigstore/cosign/v2/pkg/oci/remote"
	walk "github.com/sigstore/cosign/v2/pkg/oci/walk"
)

// synchronizer holds all the information to perform cosign copies across different registries.
type synchronizer struct {
	repoImage      *repoImage
	sourceRegistry *sourceRegistry
	primeRegistry  *primeRegistry
	nameOpts       []name.Option
}

// repoImage holds the source and destination image/tag informations
type repoImage struct {
	srcRef           name.Reference
	dstRef           name.Reference
	srcRootRefDigest name.Digest
	srcRoot          *remote.Descriptor
	srcDstMap        map[*remote.Descriptor]name.Tag
}

// sourceRegistry will be either Staging Registry or Docker Hub
type sourceRegistry struct {
	ociOpts []ociremote.Option
}

type primeRegistry struct {
	pusher     *remote.Pusher
	remoteOpts []remote.Option
}

var (
	// uaString is meant to resemble the User-Agent sent by browsers with requests.
	// See: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/User-Agent
	uaString = fmt.Sprintf("cosign/%s (%s; %s)", version.GetVersionInfo().GitVersion, runtime.GOOS, runtime.GOARCH)
)

type tagMap func(name.Reference, ...ociremote.Option) (name.Tag, error)

// Sync will load the sync yaml files and iterate through each image/tags copying and pushing without overwriting anything.
// There can be 2 sources:
//   - Docker Hub
//   - Staging Registry
//
// There is only one destination:
//   - Prime Registry
func Sync(ctx context.Context, username, password string) error {
	s, err := prepareSync(ctx, username, password)
	if err != nil {
		return err
	}

	stagingImageTags, err := loadSyncYamlFile(ctx, path.StagingToPrimeSync)
	if err != nil {
		return err
	}

	dockerImageTags, err := loadSyncYamlFile(ctx, path.DockerToPrimeSync)
	if err != nil {
		return err
	}

	stagingImageTags = sanitizeTags(stagingImageTags)
	dockerImageTags = sanitizeTags(dockerImageTags)

	if len(stagingImageTags) == 0 && len(dockerImageTags) == 0 {
		logger.Log(ctx, slog.LevelInfo, "nothing to sync")
		return nil
	}

	// Staging to Prime Registry
	if len(stagingImageTags) > 0 {
		logger.Log(ctx, slog.LevelInfo, "sync images from Staging Registry", slog.Any("staging", stagingImageTags))

		for repo, tags := range stagingImageTags {
			for _, tag := range tags {
				s.repoImage = &repoImage{} // init/reset img/tag to be synced
				if err := s.copy(ctx, StagingURL, repo, tag); err != nil {
					return err
				}
				if err := s.push(ctx); err != nil {
					return err
				}
			}
		}
	}
	// Docker to Prime Registry
	if len(dockerImageTags) > 0 {
		logger.Log(ctx, slog.LevelInfo, "sync images from Docker Hub", slog.Any("docker", dockerImageTags))

		for repo, tags := range dockerImageTags {
			for _, tag := range tags {
				s.repoImage = &repoImage{}
				if err := s.copy(ctx, DockerURL, repo, tag); err != nil {
					return err
				}
				if err := s.push(ctx); err != nil {
					return err
				}
			}
		}
	}

	logger.Log(ctx, slog.LevelInfo, "sync process complete")
	return nil
}

// prepareSync checks if the prime credentials are provided and creates the synchronizer
// with all the oci,naming and remote options needed.
func prepareSync(ctx context.Context, username, password string) (*synchronizer, error) {
	// Use strict validation for pulling and pushing
	// These options control how image references (e.g., "myregistry/myimage:tag")
	// are parsed and validated by go-containerregistry's 'name' package.
	var nameOpts []name.Option
	nameOpts = append(nameOpts, name.StrictValidation)
	nameOpts = append(nameOpts, name.Insecure)
	// name.Insecure: Allows parsing of image references that might imply an insecure
	// connection, such as "localhost:5000" or docker.io without loging.
	// This affects the parsing phase, not the actual network transport.

	// handle insecure (HTTP or self-signed HTTPS) registry connections.
	// (needed for docker.io without login)
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
	}

	// applied to the puller and subsequently used by cosign's oci/remote
	// package when fetching signed entities.
	clientOpts := []remote.Option{
		remote.WithContext(ctx),
		remote.WithUserAgent(uaString),
		remote.WithAuthFromKeychain(authn.DefaultKeychain),
		remote.WithTransport(tr),
	}

	// reuse new remote puller for efficiency across operations.
	// puller interacts only with docker.io and staging registry.
	puller, err := remote.NewPuller(clientOpts...)
	if err == nil {
		clientOpts = append(clientOpts, remote.Reuse(puller))
	}

	// These options are specifically for cosign's 'pkg/oci/remote' functions
	// (e.g., ociremote.SignedEntity, ociremote.SignatureTag). They bridge the
	// 'go-containerregistry' remote options to cosign's operations.
	pullerOpts := []ociremote.Option{
		ociremote.WithNameOptions(nameOpts...),
	}
	// Embed the 'go-containerregistry' remote options
	// (context, user agent, keychain auth, insecure transport) into cosign's client setup.
	pullerOpts = append(pullerOpts, ociremote.WithRemoteOptions(clientOpts...))

	// prime (destination) registry. They use explicit basic authentication?
	remoteOpts := []remote.Option{
		remote.WithContext(ctx),
		remote.WithAuth(&authn.Basic{Username: username, Password: password}),
	}

	// Create a new remote pusher with the prime registry's specific authentication.
	pusher, err := remote.NewPusher(remoteOpts...)
	if err != nil {
		return nil, err
	}

	return &synchronizer{
		nameOpts: nameOpts,
		sourceRegistry: &sourceRegistry{
			ociOpts: pullerOpts,
		},
		primeRegistry: &primeRegistry{
			pusher:     pusher,
			remoteOpts: remoteOpts,
		},
	}, nil
}

// loadSyncYamlFile will load a given sync registry yaml file located at config/ dir
func loadSyncYamlFile(ctx context.Context, path string) (map[string][]string, error) {
	yamlData, err := filesystem.LoadYamlFile[map[string][]string](ctx, path, true)
	if err != nil {
		return nil, err
	}

	if yamlData == nil {
		return map[string][]string{}, nil
	}

	return *yamlData, nil
}

// copy calculates the proper reference for the given img/tag at source and destination.
// pulls in memory the signatures (if any) and the entity itself.
func (s *synchronizer) copy(ctx context.Context, registry, repo, tag string) error {
	logger.Log(ctx, slog.LevelInfo, "cosign check/copy to Prime",
		slog.String("registry", registry),
		slog.String("repository", repo),
		slog.String("tag", tag))

	// Build targets
	srcTarget := registry + repo + ":" + tag
	dstTarget := PrimeURL + repo + ":" + tag

	srcRef, err := name.ParseReference(srcTarget, s.nameOpts...)
	if err != nil {
		return err
	}
	dstRef, err := name.ParseReference(dstTarget, s.nameOpts...)
	if err != nil {
		return err
	}

	s.repoImage.srcRef = srcRef
	s.repoImage.dstRef = dstRef

	if registry == StagingURL {
		return s.pullFromStaging(ctx, srcTarget)
	}
	return s.pullFromDocker(ctx, srcTarget)
}

// pullFromDocker fetches the primary image manifest from Docker Hub,
// and prepares its details for synchronization. Unlike 'pullFromStaging', this function
// specifically focuses on the main image artifact and does not explicitly discover or
// process associated cosign artifacts (signatures, attestations, SBOMs) for copying.
// It populates 's.repoImage.srcRootRefDigest' and 's.repoImage.srcRoot' for the main image.
//
// Parameters:
//
//	ctx: The context for the operation.
//	imgRepo: The repository path of the image (e.g., "rancher/rancher-webhook").
//
// Returns:
//
//	An error if the main image's manifest cannot be fetched, its digest cannot be obtained,
//	or if the pre-copy check fails.
func (s *synchronizer) pullFromDocker(ctx context.Context, imgRepo string) error {
	// Get the base repository reference for the source image.
	srcRepoRef := s.repoImage.srcRef.Context()

	root, err := ociremote.SignedEntity(s.repoImage.srcRef, s.sourceRegistry.ociOpts...)
	if err != nil {
		logger.Log(ctx, slog.LevelError, "signedEntity failure", logger.Err(err))
		return err
	}
	// Get the unique cryptographic digest of the root (main image) manifest.
	// Every OCI artifact inherently has a digest;
	h, err := root.Digest()
	if err != nil {
		return err
	}

	sourceDigest := srcRepoRef.Digest(h.String())
	// Perform a check/fetch for the main image to determine if it needs to be copied.
	got, err := s.fetchSourceCheckDestination(ctx, sourceDigest, s.repoImage.dstRef)
	if err != nil {
		return err
	}
	if got != nil {
		s.repoImage.srcRootRefDigest = sourceDigest
		s.repoImage.srcRoot = got
	}

	return nil
}

// pullFromStaging identifies all image artifacts (including signatures, attestations, and SBOMs)
// associated with a source image in the staging registry that need to be copied
// to the destination registry. It populates the synchronizer's internal maps
// with these discovered artifacts for subsequent copying.
//
// Parameters:
//
//	ctx: The context for the operation.
//	imgRepo: The repository path of the image (e.g., "rancher/fleet-agent").
//
// Returns:
//
//	An error if any step of discovery or internal state update fails.
//	On success, 's.repoImage.srcDstMap' will be populated with artifacts to copy,
//	and 's.repoImage.srcRootRefDigest' / 's.repoImage.srcRoot' will hold details
//	of the main image manifest.
func (s *synchronizer) pullFromStaging(ctx context.Context, imgRepo string) error {
	// srcDstMap will temporarily store a mapping of the source artifact's descriptor
	// (what was actually fetched from the source) to its intended destination tag.
	srcDstMap := make(map[*remote.Descriptor]name.Tag, 0)
	// 'tags' is a slice of cosign-specific artifacts based on an image's digest.
	//   - ociremote.SignatureTag:   Points to the cryptographic signature of the image manifest itself (.sig).
	//   - ociremote.AttestationTag: Points to a cryptographically signed statement *about* the image (.att).
	//   - ociremote.SBOMTag:        Software Bill of Materials (.sbom), all software components and dependencies within the image.
	tags := []tagMap{ociremote.SignatureTag, ociremote.AttestationTag, ociremote.SBOMTag}

	srcRepoRef := s.repoImage.srcRef.Context()
	dstRepoRef := s.repoImage.dstRef.Context()

	// An oci.SignedEntity represents the image manifest itself AND all its associated
	root, err := ociremote.SignedEntity(s.repoImage.srcRef, s.sourceRegistry.ociOpts...)
	if err != nil {
		logger.Log(ctx, slog.LevelError, "signedEntity failure", logger.Err(err))
		return err
	}

	logger.Log(ctx, slog.LevelInfo, "checking tags and digests to copy")
	// walk.SignedEntity traverses the 'root' entity and its children (signatures, attestations).
	if err := walk.SignedEntity(ctx, root, func(ctx context.Context, se oci.SignedEntity) error {
		// cryptographic digest of the current entity.
		h, err := se.Digest()
		if err != nil {
			return err
		}

		srcDigest := srcRepoRef.Digest(h.String())
		copyTag := func(tm tagMap) error {
			// Construct the *expected name* (e.g., "myimage:sha256-digest.sig")
			// for the artifact based on its digest. This call does NOT confirm existence yet.
			src, err := tm(srcDigest, s.sourceRegistry.ociOpts...)
			if err != nil {
				return err
			}

			// Create the corresponding destination tag reference.
			dst := dstRepoRef.Tag(src.Identifier())
			// 'got' is a *remote.Descriptor of the artifact (if it needs to be copied).
			//   - Checking if 'src' really exists.
			//   - Checking if 'dst' already exists.
			got, err := s.fetchSourceCheckDestination(ctx, src, dst)
			if err != nil {
				return err
			}
			if got != nil {
				srcDstMap[got] = dst
			}

			return nil
		}

		for _, tag := range tags {
			if err := copyTag(tag); err != nil {
				return err
			}
		}

		// Handle the main image manifest itself, or any other OCI entity
		// that is not specifically a .sig, .att, or .sbom tag.
		// It copies the entity by its digest-based tag.
		dst := dstRepoRef.Tag(srcDigest.Identifier())
		dst = dst.Tag(fmt.Sprint(h.Algorithm, "-", h.Hex))
		got, err := s.fetchSourceCheckDestination(ctx, srcDigest, dst)
		if err != nil {
			return err
		}
		if got != nil {
			srcDstMap[got] = dst
		}

		return nil
	}); err != nil {
		return err
	}

	// After walking all entities, retrieve the digest of the root (main image) manifest.
	h, err := root.Digest()
	if err != nil {
		return err
	}

	// Create a reference to the source's root image by its digest.
	sourceDigest := srcRepoRef.Digest(h.String())

	// Perform a final check/fetch for the root image to ensure it's captured in srcDstMap.
	// This specifically uses the original destination reference (s.repoImage.dstRef)
	// which holds the user-friendly tag (e.g., "myrepo/myimage:v1.0").
	// This ensures the main image gets its intended final tag after all artifacts are copied.
	got, err := s.fetchSourceCheckDestination(ctx, sourceDigest, s.repoImage.dstRef)
	if err != nil {
		return err
	}
	// Handle an abnormal scenario where the main image (root entity) could not be fetched/validated,
	// but associated artifacts (signatures, attestations) were found.
	// This indicates an inconsistent state in the source, as the primary artifact is missing.
	if got == nil && len(srcDstMap) > 0 {
		logger.Log(ctx, slog.LevelWarn, "could not fetch the entity but received tags, not syncing this!")
		return nil
	}
	// Store the root image's digest and descriptor in the synchronizer's state.
	if got != nil {
		s.repoImage.srcRootRefDigest = sourceDigest
		s.repoImage.srcRoot = got
	}
	// Store the populated map of artifacts (descriptor -> destination tag)
	s.repoImage.srcDstMap = srcDstMap
	return nil
}

// fetchSourceCheckDestination will try to fetch the built source and check if it is present on the destionation.
// The source may not exist when fetching for an inexistent triangulated signature.
// If the destination already exists, we never overwrite it.
// 404s on the source image are not errors but won't sync the tag
// trying many flavors of tag (sig, sbom, att) and only a subset of these are likely to exist, e.g., multi-arch image.
// 404s on the destination image are not errors and is the expected values.
// 200s on the destination image means the image should not be overwritten
func (s *synchronizer) fetchSourceCheckDestination(ctx context.Context, src, dst name.Reference) (*remote.Descriptor, error) {
	srcDescriptor, err := s.getRemoteSource(ctx, src)
	if err != nil {
		return nil, err
	}
	if srcDescriptor == nil {
		return nil, nil
	}

	dstExist, err := s.checkPrimeTagExist(ctx, dst)
	if err != nil {
		return nil, err
	}

	// if destination exists, it can't be overwritten.
	// if destination does not exist, it must be synced.
	if dstExist {
		logger.Log(ctx, slog.LevelWarn, "already exists, do not overwrite it", slog.String("src", src.Identifier()))
		return nil, nil
	}
	return srcDescriptor, nil
}

// getRemoteSource will attempt to pull a given img/tag and return it.
func (s *synchronizer) getRemoteSource(ctx context.Context, src name.Reference) (*remote.Descriptor, error) {

	got, err := remote.Get(src)
	if err != nil {
		var te *transport.Error
		if errors.As(err, &te) && te.StatusCode == http.StatusNotFound {
			// 404 is not treated as an error, but the img/tag descriptor is a nil return
			// therefore it will not be synced since it does not exist at source registry.
			return nil, nil
		}

		logger.Log(ctx, slog.LevelError, "failure to check source tag", logger.Err(err))
		return nil, err
	}

	logger.Log(ctx, slog.LevelInfo, "copying", slog.String("img", src.Name()))
	return got, nil
}

// checkPrimeTagExist checks if a given source already exists at the Prime Registry
func (s *synchronizer) checkPrimeTagExist(ctx context.Context, dst name.Reference) (bool, error) {
	exist := true

	_, err := remote.Head(dst, s.primeRegistry.remoteOpts...)
	if err != nil {
		exist = false

		var te *transport.Error
		if errors.As(err, &te) && te.StatusCode == http.StatusNotFound {
			// 404s are not treated as errors, means the img/tag does not exist
			err = nil
		} else {
			logger.Log(ctx, slog.LevelError, "failure to check prime tag", logger.Err(err))
		}
	}

	logger.Log(ctx, slog.LevelDebug, "checking", slog.Bool("exist", exist), slog.String("dst", dst.Name()))
	return exist, err
}

// push will iterate through repoImage source destination map and push the copied image/tags and the entity itself
func (s *synchronizer) push(ctx context.Context) error {
	// Never overwrite it
	if s.repoImage.srcRoot == nil && len(s.repoImage.srcDstMap) == 0 {
		logger.Log(ctx, slog.LevelInfo, "don't push, already synced", slog.String("img", s.repoImage.dstRef.Identifier()))
		return nil
	}

	// pushing signatures, attestations and digests if any
	for src, dst := range s.repoImage.srcDstMap {
		logger.Log(ctx, slog.LevelInfo, "pushing", slog.String("tag", dst.String()))
		if err := s.primeRegistry.pusher.Push(ctx, dst, src); err != nil {
			logger.Log(ctx, slog.LevelError, "push failed", slog.String("tag", dst.String()))
			return err
		}
	}

	// avoid a panic and last check for not overwriting anything
	if s.repoImage.srcRoot == nil {
		logger.Log(ctx, slog.LevelInfo, "don't push, already synced", slog.String("img", s.repoImage.dstRef.Identifier()))
		return nil
	}

	// pushing image/tag entity
	logger.Log(ctx, slog.LevelInfo, "pushing", slog.String("img", s.repoImage.dstRef.Identifier()))
	if err := s.primeRegistry.pusher.Push(ctx, s.repoImage.dstRef, s.repoImage.srcRoot); err != nil {
		logger.Log(ctx, slog.LevelError, "push failed", slog.String("img", s.repoImage.dstRef.Name()))
		return err
	}

	return nil
}
