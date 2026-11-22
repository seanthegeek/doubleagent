# PSScriptAnalyzer settings for the round pipeline's guest scripts under
# lab/, which run in the Windows lab VM under Windows PowerShell 5.1.
# Run from the repository root with PSScriptAnalyzer 1.25.0:
#   Invoke-ScriptAnalyzer -Path lab/lib/drive.ps1 -Settings lab/PSScriptAnalyzerSettings.psd1
#
# The default rule set at Error and Warning severity, plus the compatibility
# rules, which check that the scripts run under Windows PowerShell 5.1 as well
# as PowerShell 7.
@{
    Severity     = @('Error', 'Warning')
    IncludeDefaultRules = $true

    ExcludeRules = @(
        # The functions are internal helpers of scripts run over ssh, never
        # exported; a plural noun (Send-Keys) says the function takes a list.
        'PSUseSingularNouns'
    )

    Rules = @{
        PSUseCompatibleSyntax = @{
            Enable         = $true
            TargetVersions = @('5.1', '7.4')
        }
        # Profile names are the files PSScriptAnalyzer 1.25.0 ships under
        # compatibility_profiles/:
        #   win-8_x64_10.0.14393.0_5.1.14393.2791_x64_4.0.30319.42000_framework
        #     Windows Server 2016, Windows PowerShell 5.1
        #   win-48_x64_10.0.17763.0_5.1.17763.316_x64_4.0.30319.42000_framework
        #     Windows 10 (1809), Windows PowerShell 5.1
        PSUseCompatibleCommands = @{
            Enable         = $true
            TargetProfiles = @(
                'win-8_x64_10.0.14393.0_5.1.14393.2791_x64_4.0.30319.42000_framework',
                'win-48_x64_10.0.17763.0_5.1.17763.316_x64_4.0.30319.42000_framework'
            )
        }
        PSUseCompatibleTypes = @{
            Enable         = $true
            TargetProfiles = @(
                'win-8_x64_10.0.14393.0_5.1.14393.2791_x64_4.0.30319.42000_framework',
                'win-48_x64_10.0.17763.0_5.1.17763.316_x64_4.0.30319.42000_framework'
            )
        }
        PSUseCompatibleCmdlets = @{
            Compatibility = @('desktop-5.1.14393.206-windows')
        }
    }
}
