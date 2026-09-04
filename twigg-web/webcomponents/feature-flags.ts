// Copy of go-code featureflags.Flags
export type FeatureFlags = {
    RepoSecretsIsEnabled: boolean
    CreateCdJobs: boolean
    ShowCdJobs: boolean
    OrganizationFeatureIsEnabled: boolean
    ShowCommitSize: boolean
    EnableUserEducation: boolean
    DummyFlag: boolean
    UseVSCodeDiff: boolean
}
export function GetFeatureFlags(): FeatureFlags{
    return featureFlags
}
export function SetFeatureFlags(f: FeatureFlags){
    featureFlags = f
}
var featureFlags: FeatureFlags = {} as FeatureFlags