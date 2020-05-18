package resources

import (
	"fmt"

	jenkinsv1alpha2 "github.com/jenkinsci/kubernetes-operator/pkg/apis/jenkins/v1alpha2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	NameWithSuffixFormat         = "%s-%s"
	PluginDefinitionFormat       = "%s:%s"
	BuilderDockerfileArg         = "--dockerfile=/workspace/dockerfile/Dockerfile"
	BuilderContextDirArg         = "--context=dir://workspace/"
	BuilderPushArg               = "--no-push"
	BuilderDestinationArg        = "--destination"
	BuilderDigestFileArg         = "--digest-file=/dev/termination-log"
	BuilderSuffix                = "builder"
	DockerfileStorageSuffix      = "dockerfile-storage"
	DockerSecretSuffix           = "docker-secret"
	DockerfileNameSuffix         = "dockerfile"
	JenkinsImageBuilderImage     = "gcr.io/kaniko-project/executor:debug"
	JenkinsImageBuilderName      = "jenkins-image-builder"
	JenkinsImageDefaultBaseImage = "jenkins/jenkins:lts"
	DockerfileName               = "Dockerfile"
	DockerfileTemplate           = `FROM %s
RUN curl -o /tmp/install-plugins.sh https://raw.githubusercontent.com/jenkinsci/docker/master/install-plugins.sh
RUN chmod +x /tmp/install-plugins.sh
RUN install-plugins.sh %s `
)

var log = logf.Log.WithName("controller_jenkinsimage")

// NewBuilderPod returns a busybox pod with the same name/namespace as the cr.
func NewBuilderPod(cr *jenkinsv1alpha2.JenkinsImage) *corev1.Pod {
	logger := log.WithName("jenkinsimage_NewBuilderPod")
	name := fmt.Sprintf(NameWithSuffixFormat, cr.Name, BuilderSuffix)
	logger.Info(fmt.Sprintf("Creating a new builder pod with name %s", name))
	args := []string{BuilderDockerfileArg, BuilderContextDirArg, BuilderDigestFileArg}
	spec := cr.Spec
	destination := spec.To
	if len(destination.Registry) == 0 {
		args = append(args, BuilderPushArg)
		logger.Info(fmt.Sprintf("No Spec.Destnation.Registry found in JenkinsImage %s: push will not be performed", cr.Name))
	} else {
		destinationRegistry := fmt.Sprintf("%s=%s/%s:%s", BuilderDestinationArg, destination.Registry, destination.Name, destination.Tag)
		args = append(args, destinationRegistry)
		logger.Info(fmt.Sprintf("Builder pod will push to %s using secret %s", destinationRegistry, spec.To.Secret))
	}
	volumes := getVolumes(cr)
	volumeMounts := getVolumesMounts(cr)
	p := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cr.Namespace,
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:         JenkinsImageBuilderName,
					Image:        JenkinsImageBuilderImage,
					Args:         args,
					VolumeMounts: volumeMounts,
				},
			},
			Volumes: volumes,
		},
	}
	return p
}

// NewDockerfileConfigMap returns a busybox pod with the same name/namespace as the cr.
func NewDockerfileConfigMap(cr *jenkinsv1alpha2.JenkinsImage) *corev1.ConfigMap {
	dockerfileContent := fmt.Sprintf(DockerfileTemplate, getDefaultedBaseImage(cr), getPluginsList(cr))
	name := fmt.Sprintf(NameWithSuffixFormat, cr.Name, DockerfileNameSuffix)
	data := map[string]string{DockerfileName: dockerfileContent}
	dockerfile := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cr.Namespace,
		},
		Data: data,
	}
	return dockerfile
}

func getPluginsList(cr *jenkinsv1alpha2.JenkinsImage) string {
	logger := log.WithName("jenkinsimage_getPluginsList")
	plugins := ""
	for _, v := range cr.Spec.Plugins {
		plugins += fmt.Sprintf(PluginDefinitionFormat, v.Name, v.Version) + " "
		logger.Info(fmt.Sprintf("Adding plugin %s:%s ", v.Name, v.Version))
	}
	return plugins
}

func getDefaultedBaseImage(cr *jenkinsv1alpha2.JenkinsImage) string {
	if len(cr.Spec.From.Name) != 0 {
		return cr.Spec.From.Name
	}
	return JenkinsImageDefaultBaseImage
}

func getVolumes(cr *jenkinsv1alpha2.JenkinsImage) []corev1.Volume {
	logger := log.WithName("jenkinsimage_getVolumes")
	logger.Info(fmt.Sprintf("Creating volumes for  cr:  %s ", cr.Name))
	name := fmt.Sprintf(NameWithSuffixFormat, cr.Name, DockerfileStorageSuffix)
	storage := corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
	logger.Info(fmt.Sprintf("Storage volume of type emptyDir and name :  %s created", name))

	name = fmt.Sprintf(NameWithSuffixFormat, cr.Name, DockerfileNameSuffix)
	config := corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: name},
			},
		},
	}
	logger.Info(fmt.Sprintf("Config volume of type ConfigMap and name :  %s created", name))
	name = fmt.Sprintf(NameWithSuffixFormat, cr.Name, DockerSecretSuffix)
	secret := corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: cr.Spec.To.Secret,
			},
		},
	}
	logger.Info(fmt.Sprintf("Secret volume of type Secret using secret %s and name : %s created", cr.Spec.To.Secret, name))
	volumes := []corev1.Volume{storage, config, secret}
	return volumes
}

func getVolumesMounts(cr *jenkinsv1alpha2.JenkinsImage) []corev1.VolumeMount {
	logger := log.WithName("jenkinsimage_getVolumesMounts")
	name := fmt.Sprintf(NameWithSuffixFormat, cr.Name, DockerfileStorageSuffix)
	mountPath := "/workspace"
	storage := corev1.VolumeMount{
		Name:      name,
		MountPath: mountPath,
	}
	logger.Info(fmt.Sprintf("Volument mount for %s and mountPath : %s created", name, mountPath))

	name = fmt.Sprintf(NameWithSuffixFormat, cr.Name, DockerfileNameSuffix)
	mountPath = "/workspace/dockerfile"
	config := corev1.VolumeMount{
		Name:      name,
		MountPath: mountPath,
	}
	logger.Info(fmt.Sprintf("Volument mount for %s and mountPath : %s created", name, mountPath))

	name = fmt.Sprintf(NameWithSuffixFormat, cr.Name, DockerSecretSuffix)
	mountPath = "/kaniko/.docker/"
	secret := corev1.VolumeMount{
		Name:      name,
		MountPath: mountPath,
	}
	logger.Info(fmt.Sprintf("Volument mount for %s and mountPath : %s created", name, mountPath))

	volumeMounts := []corev1.VolumeMount{storage, config, secret}
	return volumeMounts
}