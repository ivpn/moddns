package apple

var mobileconfigTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>PayloadDisplayName</key>
	<string>modDNS</string>
	<key>PayloadDescription</key>
	<string>This configuration profile establishes secure DNS settings using modDNS service for enhanced privacy and security.</string>
	<key>PayloadIdentifier</key>
	<string>{{.PayloadIdentifier}}</string>
	<key>PayloadType</key>
	<string>Configuration</string>
	<key>PayloadUUID</key>
	<string>{{.PayloadUUID}}</string>
	<key>PayloadVersion</key>
	<integer>1</integer>
	<key>PayloadScope</key>
	<string>System</string>
	<key>PayloadContent</key>
	<array>
		<dict>
			<key>DNSSettings</key>
			{{ if or ( eq .EncryptionType "https") ( eq .EncryptionType "") }}
			<dict>
				<key>DNSProtocol</key>
				<string>HTTPS</string>
				<key>ServerURL</key>
				{{ if .DeviceId }}
				<string>https://{{.ServerDomain}}/dns-query/{{.ProfileId}}/{{ urlquery .DeviceId }}</string>
				{{ else }}
				<string>https://{{.ServerDomain}}/dns-query/{{.ProfileId}}</string>
				{{ end }}
			</dict>
			{{ end }}
			{{ if eq .EncryptionType "tls" }}
			<dict>
				<key>DNSProtocol</key>
				<string>TLS</string>
				<key>ServerAddresses</key>
				<array>
				{{- range .ServerAddresses }}
					<string>{{ . }}</string>
				{{- end }}
				</array>
				<key>ServerName</key>
				{{ if .DeviceId }}
				<string>{{.DeviceLabelEncoded}}-{{.ProfileId}}.{{.ServerDomain}}</string>
				{{ else }}
				<string>{{.ProfileId}}.{{.ServerDomain}}</string>
				{{ end }}
			</dict>
			{{ end }}
			<key>PayloadType</key>
			<string>{{ .DNSSettingsPayloadType }}</string>
			<key>PayloadIdentifier</key>
			<string>{{.DNSSettingsPayloadIdentifier}}</string>
			<key>PayloadUUID</key>
			<string>{{.DNSSettingsPayloadUUID}}</string>
			<key>PayloadDisplayName</key>
			<string>modDNS</string>
			<key>PayloadOrganization</key>
			<string>IVPN</string>
			<key>PayloadVersion</key>
			<integer>1</integer>
			<key>PayloadDescription</key>
			<string>Configures your device to use modDNS - secure DNS service.</string>
			<key>OnDemandRules</key>
			<array>
				{{/* Explicitly disconnect on excluded networks */}}
				{{ if .ExcludedWifiNetworks }}
				<dict>
					<key>Action</key>
					<string>Disconnect</string>
					<key>SSIDMatch</key>
					<array>
						{{- range .ExcludedWifiNetworks }}
						<string>{{ . }}</string>
						{{- end }}
					</array>
				</dict>
				{{ end }}

				{{/* Always connect on all other interfaces */}}
				<dict>
					<key>Action</key>
					<string>Connect</string>
				</dict>
			</array>
		</dict>
	</array>
</dict>
</plist>`

// TODO: paste this in proper place if necessary
// {{ if .ExcludedDomains }}
// <dict>
// 	<key>Action</key>
// 	<string>EvaluateConnection</string>
// 	<key>ActionParameters</key>
// 	<array>
// 		<dict>
// 			<key>DomainAction</key>
// 			<string>NeverConnect</string>
// 			<key>Domains</key>
// 				<array>
// 					{{- range .ExcludedDomains }}
// 					<string>{{ . }}</string>
// 					{{- end }}
// 				</array>
// 		</dict>
// 	</array>
// </dict>
// {{ end }}

// TODO: paste this in proper place if necessary
// <key>PayloadRemovalDisallowed</key>
// <{{ .PayloadRemovalDisallowed }}/>
