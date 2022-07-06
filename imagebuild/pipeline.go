package imagebuild

import (
	c "github.com/devfbe/gipgee/config"
	pm "github.com/devfbe/gipgee/pipelinemodel"
)

const (
	kanikoSecretsFilename = "gipgee-image-build-kaniko-auth.json" // #nosec G101
)

func GenerateReleasePipeline(config *c.Config, imagesToBuild []string, autoStart bool, gipgeeImage string) *pm.Pipeline {
	allInOneStage := pm.Stage{Name: "🏗️ All in One 🧪"}
	kanikoImage := pm.ContainerImageCoordinates{Registry: "gcr.io", Repository: "kaniko-project/executor", Tag: "debug"} // FIXME: use fixed version

	var gipgeeImageCoordinates pm.ContainerImageCoordinates

	if gipgeeImage == "" {
		gipgeeImageCoordinates = pm.ContainerImageCoordinates{
			Registry:   "docker.io",
			Repository: "devfbe/gipgee",
			Tag:        "latest",
		}
	} else {
		coords, err := pm.ContainerImageCoordinatesFromString(gipgeeImage)
		if err != nil {
			panic(err)
		}
		gipgeeImageCoordinates = *coords
	}

	generateAuthFileJob := pm.Job{
		Name:  "⚙️ Generate Kaniko docker auth file",
		Image: &gipgeeImageCoordinates,
		Stage: &allInOneStage,
		Script: []string{
			"gipgee self-release generate-kaniko-docker-auth --target staging",
		},
		Artifacts: &pm.JobArtifacts{
			Paths: []string{kanikoSecretsFilename},
		},
	}

	stagingBuildJobs := make([]*pm.Job, len(imagesToBuild))
	for idx, imageToBuild := range imagesToBuild {
		stagingBuildJobs[idx] = &pm.Job{
			Name:  "🐋 Build staging image " + imageToBuild + " using kaniko",
			Image: &kanikoImage,
			Stage: &allInOneStage,
			Script: []string{
				"mkdir -p /kaniko/.docker",
				"cp -v ${CI_PROJECT_DIR}/" + kanikoSecretsFilename + " /kaniko/.docker/config.json",
				"/kaniko/executor --context ${CI_PROJECT_DIR} --dockerfile ${CI_PROJECT_DIR}/" + *config.Images[imageToBuild].ContainerFile + " --no-push",
			},
			Needs: []pm.JobNeeds{{
				Job:       &generateAuthFileJob,
				Artifacts: true,
			}},
		}
	}

	stagingBuildJobs = append(stagingBuildJobs, &generateAuthFileJob)

	pipeline := pm.Pipeline{
		Stages: []*pm.Stage{&allInOneStage},
		Jobs:   stagingBuildJobs,
	}
	return &pipeline

}

func (params *ImageBuildCmd) Run() error {
	config, err := c.LoadConfiguration(params.ConfigFileName)
	if err != nil {
		return err
	}

	/*
		FIXME: select depending on git diff
	*/

	imagesToBuild := make([]string, 0)
	for key := range config.Images {
		imagesToBuild = append(imagesToBuild, key)
	}

	pipeline := GenerateReleasePipeline(config, imagesToBuild, true, params.GipgeeImage) // True only on manual pipeline..
	err = pipeline.WritePipelineToFile(params.PipelineFileName)
	if err != nil {
		panic(err)
	}
	return nil
}
