import {
  csvToArray
} from '../common.bicep'

extension microsoftGraphBeta

// Application identity
@description('Display name and unique name for the Entra application')
param applicationName string

@description('Comma-separated list of owner object IDs for the application and service principal')
param ownerIds string

@description('Whether to create the service principal for this application')
param manageSp bool

@description('Trusted subject name and issuer pairs for SNI authentication')
param trustedSubjectNameAndIssuers array = []

@description('Service management reference ID for the application')
param serviceManagementReference string

@description('Whether the application is a fallback public client')
param isFallbackPublicClient bool = true

@description('Requested access token version (1 or 2). Default is 2.')
param requestedAccessTokenVersion int = 2

@description('Key credentials for the application (e.g. certificate-based auth)')
param keyCredentials array = []

var validatedOwnerIds = !empty(ownerIds) ? ownerIds : fail('EntraId App ownerIds must not be empty')

resource entraApp 'Microsoft.Graph/applications@beta' = {
  displayName: applicationName
  isFallbackPublicClient: isFallbackPublicClient
  signInAudience: 'AzureADMyOrg' // Single tenant application
  uniqueName: applicationName
  requiredResourceAccess: []
  serviceManagementReference: serviceManagementReference
  api: {
    requestedAccessTokenVersion: requestedAccessTokenVersion
  }
  trustedSubjectNameAndIssuers: trustedSubjectNameAndIssuers
  owners: {
    relationships: [for ownerId in csvToArray(validatedOwnerIds): ownerId]
  }
  keyCredentials: keyCredentials
}

resource servicePrincipal 'Microsoft.Graph/servicePrincipals@beta' = if (manageSp) {
  appId: entraApp.appId
  owners: {
    relationships: [for ownerId in csvToArray(validatedOwnerIds): ownerId]
  }
}

@description('The application (client) ID')
output appId string = entraApp.appId
