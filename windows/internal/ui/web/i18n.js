"use strict";

// UI translations. The app sets window.TOLLGATE_LANG (from /lang.js) to the
// Windows display language; English text is the lookup key, so anything
// missing here stays English. {0}, {1}… are filled in by t().

const LANG = window.TOLLGATE_LANG === "tr" ? "tr" : "en";
const LOCALE = LANG === "tr" ? "tr-TR" : undefined;

const TR = {
  // Header and tabs
  "Connecting…": "Bağlanıyor…",
  "Start or stop the local proxy": "Yerel proxy'yi başlat veya durdur",
  "Proxy running": "Proxy çalışıyor",
  Overview: "Genel bakış",
  Models: "Modeller",
  Settings: "Ayarlar",
  "Running on {0}": "{0} üzerinde çalışıyor",
  "Starting…": "Başlatılıyor…",
  Stopped: "Durduruldu",
  "The engine failed to start.": "Motor başlatılamadı.",
  "Request failed ({0})": "İstek başarısız oldu ({0})",
  "CLI Tollgate is not responding.": "CLI Tollgate yanıt vermiyor.",
  "This page has expired. Open CLI Tollgate from its tray icon.":
    "Bu sayfanın süresi doldu. CLI Tollgate'i bildirim alanındaki simgesinden açın.",
  "CLI Tollgate is not running.": "CLI Tollgate çalışmıyor.",

  // Banners
  Dismiss: "Kapat",
  "CLI Tollgate {0} is ready to install.": "CLI Tollgate {0} yüklenmeye hazır.",
  "Restart to update": "Güncellemek için yeniden başlat",

  // Overview
  Accounts: "Hesaplar",
  Connect: "Bağlantı",
  "Waiting for sign in": "Giriş bekleniyor",
  Cancel: "İptal",
  "Account options": "Hesap seçenekleri",
  "Switch account…": "Hesap değiştir…",
  "Sign out": "Çıkış yap",
  "Sign in": "Giriş yap",
  "Base URL": "Temel URL",
  "base URL": "temel URL",
  "API key": "API anahtarı",
  "Copy {0}": "Kopyala: {0}",
  "Double-click to reveal": "Göstermek için çift tıklayın",
  "Show API key": "API anahtarını göster",
  "Hide API key": "API anahtarını gizle",
  "Setup snippet": "Kurulum parçası",
  "{0} setup": "{0} kurulumu",
  Copy: "Kopyala",
  "Copied {0}": "Kopyalandı: {0}",
  "Loading…": "Yükleniyor…",
  "Start the proxy to see the base URL and API key.": "Temel URL'yi ve API anahtarını görmek için proxy'yi başlatın.",
  "{0} failed requests today": "Bugün {0} başarısız istek",
  "{0} requests · {1} tokens today": "Bugün {0} istek · {1} token",

  // Access requests
  "A program on this computer, not a web page.": "Bu bilgisayardaki bir program, web sayfası değil.",
  "A page served from this computer.": "Bu bilgisayardan sunulan bir sayfa.",
  "Not a secure (https) site.": "Güvenli (https) bir site değil.",
  "1 more request waiting.": "1 istek daha bekliyor.",
  "{0} more requests waiting.": "{0} istek daha bekliyor.",
  "Allow “{0}” to use your subscriptions?": "“{0}” aboneliklerinizi kullanabilsin mi?",
  "It will be able to send requests through your signed-in ChatGPT and Claude accounts. You can revoke access anytime in Settings.":
    "Giriş yaptığınız ChatGPT ve Claude hesaplarınız üzerinden istek gönderebilecek. Erişimi istediğiniz zaman Ayarlar'dan kaldırabilirsiniz.",
  "Let “{0}” also search the web?": "“{0}” web'de de arama yapabilsin mi?",
  "Allow “{0}” to use your subscriptions and search the web?": "“{0}” aboneliklerinizi kullanabilsin ve web'de arama yapabilsin mi?",
  "This app is already connected to CLI Tollgate. It is asking for more access.":
    "Bu uygulama CLI Tollgate'e zaten bağlı. Daha fazla erişim istiyor.",
  "It will also be able to search the web and read web pages through this computer. Sites see your IP address.":
    "Bu bilgisayar üzerinden web'de arama yapıp sayfaları okuyabilecek. Siteler IP adresinizi görür.",
  Allow: "İzin ver",
  Deny: "Reddet",
  "Allowed {0}": "{0} için izin verildi",

  // Connected apps
  "Connected apps": "Bağlı uygulamalar",
  "allowed {0}": "izin verildi: {0}",
  Revoke: "Erişimi kaldır",
  "No apps yet. When a web app asks for access, CLI Tollgate asks you first.":
    "Henüz uygulama yok. Bir web uygulaması erişim istediğinde CLI Tollgate önce size sorar.",
  "Revoke access for “{0}”?": "“{0}” için erişim kaldırılsın mı?",
  "Its key stops working right away. It can ask for access again later.":
    "Anahtarı hemen çalışmayı durdurur. Daha sonra yeniden erişim isteyebilir.",
  "Access revoked": "Erişim kaldırıldı",
  Web: "Web",

  // Models
  Provider: "Sağlayıcı",
  Filter: "Filtrele",
  "Filter models": "Modelleri filtrele",
  "No models in the catalog.": "Katalogda model yok.",
  "Start the proxy to load models.": "Modelleri yüklemek için proxy'yi başlatın.",
  "No models match “{0}”.": "“{0}” ile eşleşen model yok.",
  "{0}k context": "{0}k bağlam",
  "Expose this model to clients": "Bu modeli istemcilere aç",
  "Expose {0}": "{0} modelini aç",
  alias: "takma ad",
  "Extra client-visible name. The original ID keeps working.":
    "İstemcilerin göreceği ek ad. Orijinal kimlik çalışmaya devam eder.",
  "Alias for {0}": "{0} için takma ad",
  "{0} of {1} exposed": "{0}/{1} açık",
  "Not signed in to {0}": "{0} hesabına giriş yapılmadı",
  "· not signed in": "· giriş yapılmadı",
  "Hide all": "Tümünü gizle",
  "Show all": "Tümünü göster",

  // Settings
  General: "Genel",
  "Launch at sign-in": "Oturum açıldığında başlat",
  Proxy: "Proxy",
  Port: "Port",
  Apply: "Uygula",
  "Listens on": "Dinlediği adres",
  "127.0.0.1 only": "Yalnızca 127.0.0.1",
  "Regenerate…": "Yeniden oluştur…",
  "Web pages can call the proxy (CORS is open), but every request needs the API key.":
    "Web sayfaları proxy'yi çağırabilir (CORS açık), ancak her istek API anahtarı gerektirir.",
  "Revoking an app turns off its key right away. Regenerating your API key leaves connected apps working.":
    "Bir uygulamanın erişimini kaldırmak anahtarını hemen devre dışı bırakır. API anahtarınızı yeniden oluşturmak bağlı uygulamaları etkilemez.",
  "Web access": "Web erişimi",
  "Let connected apps search and read the web": "Bağlı uygulamaların web'de arama yapıp sayfaları okumasına izin ver",
  "{0} API key": "{0} API anahtarı",
  "Get a key": "Anahtar al",
  "Default search": "Varsayılan arama",
  "Add an API key to enable search. Reading pages works without one.":
    "Aramayı etkinleştirmek için bir API anahtarı ekleyin. Sayfaları okumak anahtarsız da çalışır.",
  "Site list": "Site listesi",
  "Block listed sites": "Listedeki siteleri engelle",
  "Allow only listed sites": "Yalnızca listedeki sitelere izin ver",
  "One site per line, e.g. example.com": "Her satıra bir site, örn. example.com",
  "Searches and page reads go out from this computer, so sites see your IP address. A site entry also covers its subdomains.":
    "Aramalar ve sayfa okumaları bu bilgisayardan yapılır, bu yüzden siteler IP adresinizi görür. Bir site girdisi alt alan adlarını da kapsar.",
  Updates: "Güncellemeler",
  "Check for updates automatically": "Güncellemeleri otomatik denetle",
  "Install updates when I quit": "Çıkarken güncellemeleri yükle",
  "Downloaded updates are verified with an Ed25519 signature.": "İndirilen güncellemeler Ed25519 imzasıyla doğrulanır.",
  "Install {0}": "{0} sürümünü yükle",
  "Updates are only available in released builds.": "Güncellemeler yalnızca yayımlanmış sürümlerde kullanılabilir.",
  "Check now": "Şimdi denetle",
  "Advanced: Cloaking": "Gelişmiş: Kamuflaj",
  "Cloak requests": "İstekleri kamufle et",
  "Claude models go out as Claude Code, GPT / Codex as Codex CLI.":
    "Claude modelleri Claude Code, GPT / Codex ise Codex CLI olarak gönderilir.",
  "Requests keep the calling client's identity.": "İstekler, çağıran istemcinin kimliğini korur.",
  "Partially on. Adjust each side below.": "Kısmen açık. Her tarafı aşağıdan ayarlayın.",
  "Disguise Claude requests as Claude Code": "Claude isteklerini Claude Code gibi göster",
  "Send official Codex CLI headers for GPT / Codex": "GPT / Codex için resmi Codex CLI başlıklarını gönder",
  "Send official Codex CLI headers": "Resmi Codex CLI başlıklarını gönder",
  "Cloak Claude model IDs in {0}": "{0} içindeki Claude model kimliklerini kamufle et",
  "Cloak Claude model IDs": "Claude model kimliklerini kamufle et",
  "The disguise follows the requested model. Requests that already come from Claude Code are passed through unchanged.":
    "Kamuflaj istenen modele göre seçilir. Zaten Claude Code'dan gelen istekler değiştirilmeden iletilir.",
  About: "Hakkında",
  Version: "Sürüm",
  Engine: "Motor",
  bundled: "paketle birlikte",
  "Data folder": "Veri klasörü",
  Open: "Aç",
  "Engine log": "Motor günlüğü",
  "Restart engine": "Motoru yeniden başlat",
  "Quit CLI Tollgate": "CLI Tollgate'ten çık",
  Quit: "Çık",
  "Restarting on port {0}…": "{0} portunda yeniden başlatılıyor…",
  "Regenerate the API key?": "API anahtarı yeniden oluşturulsun mu?",
  "Clients using the current key will stop working until you update them. Connected apps keep their own keys.":
    "Mevcut anahtarı kullanan istemciler, siz onları güncelleyene kadar çalışmaz. Bağlı uygulamalar kendi anahtarlarını korur.",
  Regenerate: "Yeniden oluştur",
  "New API key created": "Yeni API anahtarı oluşturuldu",
};

// t("Running on {0}", url): translate, then fill the numbered slots.
function t(text, ...args) {
  const s = (LANG === "tr" && TR[text]) || text;
  return s.replace(/\{(\d+)\}/g, (m, i) => (i < args.length ? String(args[i]) : m));
}

// Static markup in index.html opts in with data-i18n (text content),
// data-i18n-title and data-i18n-aria-label.
function translateStatic() {
  document.documentElement.lang = LANG;
  document.querySelectorAll("[data-i18n]").forEach((el) => (el.textContent = t(el.textContent.trim())));
  document.querySelectorAll("[data-i18n-title]").forEach((el) => (el.title = t(el.title)));
  document.querySelectorAll("[data-i18n-aria-label]").forEach((el) => el.setAttribute("aria-label", t(el.getAttribute("aria-label"))));
}
