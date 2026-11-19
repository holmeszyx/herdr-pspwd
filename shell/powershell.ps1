if ($env:HERDR_ENV -eq '1' -and -not $global:__HerdrPspwdPrompt) {
    $global:__HerdrPspwdPrompt = $function:prompt
    function global:prompt {
        $loc = $ExecutionContext.SessionState.Path.CurrentLocation
        if ($loc.Provider.Name -eq 'FileSystem') {
            $parts = ($loc.ProviderPath -replace '\\', '/') -split '/' | ForEach-Object { [uri]::EscapeDataString($_) -replace '%3A', ':' }
            [Console]::Write([char]27 + ']7;file:///' + ($parts -join '/') + [char]7)
        }
        & $global:__HerdrPspwdPrompt
    }
}
