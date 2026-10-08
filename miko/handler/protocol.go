package handler

import (
	"time"

	miko "github.com/Hayao0819/Kamisato/miko/client"
	"github.com/Hayao0819/Kamisato/miko/domain"
)

func domainBuildRequest(request *miko.BuildRequest) *domain.BuildRequest {
	if request == nil {
		return nil
	}
	var git *domain.GitSource
	if request.Git != nil {
		git = &domain.GitSource{
			URL:    request.Git.URL,
			Ref:    request.Git.Ref,
			Subdir: request.Git.Subdir,
		}
	}
	return &domain.BuildRequest{
		Repo:          request.Repo,
		Arch:          request.Arch,
		Microarch:     request.Microarch,
		Git:           git,
		Pkgbuild:      request.Pkgbuild,
		Files:         request.Files,
		InstallPkgs:   request.InstallPkgs,
		SignMode:      request.SignMode,
		Timeout:       request.Timeout,
		IgnoreArch:    request.IgnoreArch,
		RunCheck:      request.RunCheck,
		RunVerify:     request.RunVerify,
		SkipChecksums: request.SkipChecksums,
		SkipPGPCheck:  request.SkipPGPCheck,
	}
}

func protocolBuildJob(job *domain.BuildJob) miko.BuildJob {
	return miko.BuildJob{
		ID:        job.ID,
		Repo:      job.Repo,
		Arch:      job.Arch,
		Status:    miko.JobStatus(job.Status),
		Logs:      job.Logs,
		Err:       job.Err,
		Packages:  job.Packages,
		Retries:   job.Retries,
		Reason:    miko.BuildReason(job.Reason),
		CreatedAt: job.CreatedAt.Format(time.RFC3339Nano),
		StartedAt: protocolTime(job.StartedAt),
		EndedAt:   protocolTime(job.EndedAt),
	}
}

func protocolBuildJobs(jobs []*domain.BuildJob) []miko.BuildJob {
	result := make([]miko.BuildJob, 0, len(jobs))
	for _, job := range jobs {
		result = append(result, protocolBuildJob(job))
	}
	return result
}

func protocolBuildStats(stats domain.BuildStats) miko.BuildStats {
	counts := make(map[miko.JobStatus]int, len(stats.Counts))
	for status, count := range stats.Counts {
		counts[miko.JobStatus(status)] = count
	}
	return miko.BuildStats{
		Workers:     stats.Workers,
		QueueLength: stats.QueueLength,
		Running:     stats.Running,
		Counts:      counts,
		Total:       stats.Total,
		SuccessRate: stats.SuccessRate,
		UptimeSec:   int64(stats.UptimeSec),
	}
}

func protocolTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.Format(time.RFC3339Nano)
	return &formatted
}
