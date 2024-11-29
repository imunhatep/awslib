package rds

import (
	"github.com/Masterminds/semver"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/imunhatep/gocollection/slice"
	"github.com/rs/zerolog/log"
)

type DbEngineName string

func (r *RdsRepository) FindMinimalDBEngineVersions(engine DbEngineName) (types.DBEngineVersion, error) {
	query := &rds.DescribeDBEngineVersionsInput{Engine: aws.String(string(engine))}
	versions, err := r.ListDBEngineVersionsByInput(query)
	if err != nil {
		return types.DBEngineVersion{}, errors.New(err)
	}

	minVer := slice.FoldLeft(versions, types.DBEngineVersion{}, compareDBEngineVersions)

	return minVer, nil
}

func (r *RdsRepository) ListDBEngineVersionsByInput(query *rds.DescribeDBEngineVersionsInput) ([]types.DBEngineVersion, error) {
	var versions []types.DBEngineVersion

	p := rds.NewDescribeDBEngineVersionsPaginator(r.client.RDS(), query)
	for p.HasMorePages() {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.With(r.promLabels("DescribeDBEngineVersions", ccfg.ResourceTypeDBEngineVersion)).Inc()
		}

		resp, err := p.NextPage(r.ctx)
		if err != nil {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.With(r.promLabels("DescribeDBEngineVersions", ccfg.ResourceTypeDBEngineVersion)).Inc()
			}

			return versions, errors.New(err)
		}

		versions = append(versions, resp.DBEngineVersions...)
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("ListDBEngineVersions", ccfg.ResourceTypeDBEngineVersion)).
			Add(float64(len(versions)))
	}

	return versions, nil
}

func compareDBEngineVersions(z types.DBEngineVersion, i int, v types.DBEngineVersion) types.DBEngineVersion {
	log.Trace().
		Str("left", aws.ToString(z.EngineVersion)).
		Str("right", aws.ToString(v.EngineVersion)).
		Msg("[RdsRepository.FindMinimalDBEngineVersions] comparing dbengine versions")

	rightVer, e := semver.NewVersion(aws.ToString(v.EngineVersion))
	if e != nil {
		log.Error().Err(e).Msg("[RdsRepository.FindMinimalDBEngineVersions] failed to parse version")
		return z
	}

	// set initial version
	if z.EngineVersion == nil {
		// patched dbEngines, seems indicate extended support
		if rightVer.Prerelease() == "" {
			return v
		}
		return z
	}

	// reset any previous found version, if there is same major version with a patch
	leftVer, _ := semver.NewVersion(aws.ToString(z.EngineVersion))
	if rightVer.Prerelease() != "" && leftVer.Major() == rightVer.Major() {
		return types.DBEngineVersion{}
	}

	if leftVer.GreaterThan(leftVer) {
		return v
	}

	return z
}
