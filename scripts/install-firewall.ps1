# 放行串口网关 TCP 端口（需管理员权限）
$ruleName = "SerialGateway-TCP"
$ports = "2001-2006"

$existing = Get-NetFirewallRule -DisplayName $ruleName -ErrorAction SilentlyContinue
if ($existing) {
    Write-Host "Firewall rule '$ruleName' already exists."
} else {
    New-NetFirewallRule -DisplayName $ruleName -Direction Inbound `
        -Protocol TCP -LocalPort $ports -Action Allow | Out-Null
    Write-Host "Created firewall rule '$ruleName' for TCP $ports"
}
