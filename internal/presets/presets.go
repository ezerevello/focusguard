// Package presets contains predefined domain lists for the most common services people want to block. They cover both the web version and, where applicable, domains used by desktop/mobile apps based on Electron or WebView (e.g., WhatsApp Desktop).
package presets

// Preset is an entry ready to be added with a single click from the web UI.
type Preset struct {
	Key     string   `json:"key"`
	Name    string   `json:"name"`
	Domains []string `json:"domains"`
}

// All returns the full list of available presets.
func All() []Preset {
	return []Preset{
		{
			Key:  "youtube",
			Name: "YouTube",
			Domains: []string{
				"youtube.com", "youtu.be", "m.youtube.com",
				"youtube-nocookie.com", "googlevideo.com",
			},
		},
		{
			Key:  "whatsapp",
			Name: "WhatsApp (Web + Desktop)",
			Domains: []string{
				"web.whatsapp.com", "whatsapp.com", "whatsapp.net",
			},
		},
		{
			Key:  "instagram",
			Name: "Instagram",
			Domains: []string{
				"instagram.com", "cdninstagram.com",
			},
		},
		{
			Key:  "tiktok",
			Name: "TikTok",
			Domains: []string{
				"tiktok.com", "tiktokcdn.com", "tiktokv.com",
			},
		},
		{
			Key:  "facebook",
			Name: "Facebook",
			Domains: []string{
				"facebook.com", "fb.com", "fbcdn.net", "messenger.com",
			},
		},
		{
			Key:  "twitter",
			Name: "X / Twitter",
			Domains: []string{
				"twitter.com", "x.com", "t.co",
			},
		},
		{
			Key:  "reddit",
			Name: "Reddit",
			Domains: []string{
				"reddit.com", "redd.it",
			},
		},
		{
			Key:  "netflix",
			Name: "Netflix",
			Domains: []string{
				"netflix.com", "nflxvideo.net",
			},
		},
		{
			Key:  "twitch",
			Name: "Twitch",
			Domains: []string{
				"twitch.tv", "ttvnw.net",
			},
		},
		{
			Key:  "discord",
			Name: "Discord",
			Domains: []string{
				"discord.com", "discordapp.com", "discord.gg",
			},
		},
	}
}
