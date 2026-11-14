package i18n

// turkish holds the Go-side strings (tray menu, status and error messages).
// The web UI has its own table in internal/ui/web/i18n.js.
var turkish = map[string]string{
	// Accounts
	"Plus / Pro subscription via Codex":  "Codex üzerinden Plus / Pro aboneliği",
	"Pro / Max subscription":             "Pro / Max aboneliği",
	"Finish signing in in your browser…": "Girişi tarayıcınızda tamamlayın…",
	"Disabled":                           "Devre dışı",
	"Cooling down (rate limited)":        "Bekleme süresinde (hız sınırı)",
	"Login timed out.":                   "Giriş zaman aşımına uğradı.",
	"Login failed.":                      "Giriş başarısız oldu.",
	"could not open the browser: %w":     "tarayıcı açılamadı: %w",

	// Settings and actions
	"port must be between 1024 and 65535":               "port 1024 ile 65535 arasında olmalıdır",
	"copy failed: %w":                                   "kopyalanamadı: %w",
	"This request has expired or was already answered.": "Bu isteğin süresi doldu veya zaten yanıtlandı.",

	// Engine
	"the proxy engine is missing (%s)":         "proxy motoru bulunamadı (%s)",
	"The engine did not answer on port %d. %s": "Motor %d portunda yanıt vermedi. %s",
	"The engine exited. %s":                    "Motor kapandı. %s",
	"The engine exited (status %d). %s":        "Motor kapandı (durum %d). %s",
	"Engine returned %d: %s":                   "Motor %d döndürdü: %s",

	// Updates
	"Updates are only available in released builds.":     "Güncellemeler yalnızca yayımlanmış sürümlerde kullanılabilir.",
	"Update check failed: %s":                            "Güncelleme denetimi başarısız oldu: %s",
	"CLI Tollgate %s is the latest version.":             "CLI Tollgate %s en son sürüm.",
	"Version %s is ready to install.":                    "%s sürümü yüklenmeye hazır.",
	"no update has been downloaded":                      "henüz bir güncelleme indirilmedi",
	"no Windows release has been published yet":          "henüz bir Windows sürümü yayımlanmadı",
	"update signature is not valid; refusing to install": "güncelleme imzası geçersiz; yükleme reddedildi",

	// Setup snippets
	"Claude Code (all new terminals)":    "Claude Code (tüm yeni terminaller)",
	"OpenAI-compatible SDK (PowerShell)": "OpenAI uyumlu SDK (PowerShell)",
	"OpenAI-compatible SDK":              "OpenAI uyumlu SDK",
	"Web page (fetch)":                   "Web sayfası (fetch)",
	"curl test":                          "curl testi",

	// Tray menu
	"Open CLI Tollgate": "CLI Tollgate'i aç",
	"Models…":           "Modeller…",
	"Settings…":         "Ayarlar…",
	"Copy Base URL":     "Temel URL'yi kopyala",
	"Copy API Key":      "API anahtarını kopyala",
	"Copy Setup For":    "Kurulumu kopyala",
	"Cloak Requests":    "İstekleri kamufle et",
	"Disguise each request to match its model": "Her isteği modeline uygun şekilde kamufle et",
	"Proxy Running":                  "Proxy çalışıyor",
	"Restart Engine":                 "Motoru yeniden başlat",
	"Install Update and Restart":     "Güncellemeyi yükle ve yeniden başlat",
	"Install Update %s and Restart":  "%s güncellemesini yükle ve yeniden başlat",
	"Check for Updates…":             "Güncellemeleri denetle…",
	"Open Logs":                      "Günlükleri aç",
	"Show Data Folder":               "Veri klasörünü göster",
	"Quit CLI Tollgate":              "CLI Tollgate'ten çık",
	"Running on %s":                  "%s üzerinde çalışıyor",
	"Starting…":                      "Başlatılıyor…",
	"Stopped":                        "Durduruldu",
	"Engine failed":                  "Motor hata verdi",
	"Today: %d requests · %s tokens": "Bugün: %d istek · %s token",
	"%s: signing in…":                "%s: giriş yapılıyor…",
	"%s: Sign In…":                   "%s: Giriş yap…",
}
