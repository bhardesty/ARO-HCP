using '../templates/mise-identity.bicep'

param miseApplicationName = '{{ .mise.applicationName }}'
param entraAppOwnerIds = '{{ .entraAppOwnerIds }}'
param miseApplicationDeploy = {{ .mise.deploy }}
