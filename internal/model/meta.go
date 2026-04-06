package model

import (
	"github.com/lib/pq"
	"gorm.io/datatypes"
)

// Meta represents the `meta` table (singleton server configuration).
type Meta struct {
	ID          string  `gorm:"column:id;type:varchar(32);primaryKey" json:"id"`
	RootUserID  *string `gorm:"column:rootUserId;type:varchar(32)" json:"rootUserId"`
	Name        *string `gorm:"column:name;type:varchar(1024)" json:"name"`
	ShortName   *string `gorm:"column:shortName;type:varchar(64)" json:"shortName"`
	Description *string `gorm:"column:description;type:varchar(1024)" json:"description"`

	MaintainerName  *string `gorm:"column:maintainerName;type:varchar(1024)" json:"maintainerName"`
	MaintainerEmail *string `gorm:"column:maintainerEmail;type:varchar(1024)" json:"maintainerEmail"`

	DisableRegistration bool `gorm:"column:disableRegistration;default:true" json:"disableRegistration"`

	Langs           pq.StringArray `gorm:"column:langs;type:varchar(1024)[];default:'{}'" json:"langs"`
	PinnedUsers     pq.StringArray `gorm:"column:pinnedUsers;type:varchar(1024)[];default:'{}'" json:"pinnedUsers"`
	HiddenTags      pq.StringArray `gorm:"column:hiddenTags;type:varchar(1024)[];default:'{}'" json:"hiddenTags"`
	BlockedHosts    pq.StringArray `gorm:"column:blockedHosts;type:varchar(1024)[];default:'{}'" json:"blockedHosts"`
	SensitiveWords  pq.StringArray `gorm:"column:sensitiveWords;type:varchar(1024)[];default:'{}'" json:"sensitiveWords"`
	ProhibitedWords pq.StringArray `gorm:"column:prohibitedWords;type:varchar(1024)[];default:'{}'" json:"prohibitedWords"`
	SilencedHosts   pq.StringArray `gorm:"column:silencedHosts;type:varchar(1024)[];default:'{}'" json:"silencedHosts"`

	ThemeColor         *string `gorm:"column:themeColor;type:varchar(1024)" json:"themeColor"`
	BannerURL          *string `gorm:"column:bannerUrl;type:varchar(1024)" json:"bannerUrl"`
	BackgroundImageURL *string `gorm:"column:backgroundImageUrl;type:varchar(1024)" json:"backgroundImageUrl"`
	LogoImageURL       *string `gorm:"column:logoImageUrl;type:varchar(1024)" json:"logoImageUrl"`
	IconURL            *string `gorm:"column:iconUrl;type:varchar(1024)" json:"iconUrl"`

	CacheRemoteFiles          bool `gorm:"column:cacheRemoteFiles;default:false" json:"cacheRemoteFiles"`
	CacheRemoteSensitiveFiles bool `gorm:"column:cacheRemoteSensitiveFiles;default:true" json:"cacheRemoteSensitiveFiles"`
	EmailRequiredForSignup    bool `gorm:"column:emailRequiredForSignup;default:false" json:"emailRequiredForSignup"`

	// CAPTCHA
	EnableHcaptcha     bool    `gorm:"column:enableHcaptcha;default:false" json:"enableHcaptcha"`
	HcaptchaSiteKey    *string `gorm:"column:hcaptchaSiteKey;type:varchar(1024)" json:"hcaptchaSiteKey"`
	HcaptchaSecretKey  *string `gorm:"column:hcaptchaSecretKey;type:varchar(1024)" json:"hcaptchaSecretKey"`
	EnableRecaptcha    bool    `gorm:"column:enableRecaptcha;default:false" json:"enableRecaptcha"`
	RecaptchaSiteKey   *string `gorm:"column:recaptchaSiteKey;type:varchar(1024)" json:"recaptchaSiteKey"`
	RecaptchaSecretKey *string `gorm:"column:recaptchaSecretKey;type:varchar(1024)" json:"recaptchaSecretKey"`
	EnableTurnstile    bool    `gorm:"column:enableTurnstile;default:false" json:"enableTurnstile"`
	TurnstileSiteKey   *string `gorm:"column:turnstileSiteKey;type:varchar(1024)" json:"turnstileSiteKey"`
	TurnstileSecretKey *string `gorm:"column:turnstileSecretKey;type:varchar(1024)" json:"turnstileSecretKey"`

	// Email
	EnableEmail bool    `gorm:"column:enableEmail;default:false" json:"enableEmail"`
	Email       *string `gorm:"column:email;type:varchar(1024)" json:"email"`
	SmtpSecure  bool    `gorm:"column:smtpSecure;default:false" json:"smtpSecure"`
	SmtpHost    *string `gorm:"column:smtpHost;type:varchar(1024)" json:"smtpHost"`
	SmtpPort    *int    `gorm:"column:smtpPort" json:"smtpPort"`
	SmtpUser    *string `gorm:"column:smtpUser;type:varchar(1024)" json:"smtpUser"`
	SmtpPass    *string `gorm:"column:smtpPass;type:varchar(1024)" json:"smtpPass"`

	// Service Worker
	EnableServiceWorker bool    `gorm:"column:enableServiceWorker;default:false" json:"enableServiceWorker"`
	SwPublicKey         *string `gorm:"column:swPublicKey;type:varchar(1024)" json:"swPublicKey"`
	SwPrivateKey        *string `gorm:"column:swPrivateKey;type:varchar(1024)" json:"swPrivateKey"`

	// Object Storage
	UseObjectStorage              bool    `gorm:"column:useObjectStorage;default:false" json:"useObjectStorage"`
	ObjectStorageBucket           *string `gorm:"column:objectStorageBucket;type:varchar(1024)" json:"objectStorageBucket"`
	ObjectStoragePrefix           *string `gorm:"column:objectStoragePrefix;type:varchar(1024)" json:"objectStoragePrefix"`
	ObjectStorageBaseURL          *string `gorm:"column:objectStorageBaseUrl;type:varchar(1024)" json:"objectStorageBaseUrl"`
	ObjectStorageEndpoint         *string `gorm:"column:objectStorageEndpoint;type:varchar(1024)" json:"objectStorageEndpoint"`
	ObjectStorageRegion           *string `gorm:"column:objectStorageRegion;type:varchar(1024)" json:"objectStorageRegion"`
	ObjectStorageAccessKey        *string `gorm:"column:objectStorageAccessKey;type:varchar(1024)" json:"objectStorageAccessKey"`
	ObjectStorageSecretKey        *string `gorm:"column:objectStorageSecretKey;type:varchar(1024)" json:"objectStorageSecretKey"`
	ObjectStoragePort             *int    `gorm:"column:objectStoragePort" json:"objectStoragePort"`
	ObjectStorageUseSSL           bool    `gorm:"column:objectStorageUseSSL;default:true" json:"objectStorageUseSSL"`
	ObjectStorageUseProxy         bool    `gorm:"column:objectStorageUseProxy;default:true" json:"objectStorageUseProxy"`
	ObjectStorageSetPublicRead    bool    `gorm:"column:objectStorageSetPublicRead;default:false" json:"objectStorageSetPublicRead"`
	ObjectStorageS3ForcePathStyle bool    `gorm:"column:objectStorageS3ForcePathStyle;default:true" json:"objectStorageS3ForcePathStyle"`

	// Policies & Rules
	Policies    datatypes.JSON `gorm:"column:policies;type:jsonb;default:'{}'" json:"policies"`
	ServerRules pq.StringArray `gorm:"column:serverRules;type:varchar(280)[];default:'{}'" json:"serverRules"`

	// URLs
	TermsOfServiceURL *string `gorm:"column:termsOfServiceUrl;type:varchar(1024)" json:"termsOfServiceUrl"`
	RepositoryURL     *string `gorm:"column:repositoryUrl;type:varchar(1024)" json:"repositoryUrl"`
	FeedbackURL       *string `gorm:"column:feedbackUrl;type:varchar(1024)" json:"feedbackUrl"`
	ImpressumURL      *string `gorm:"column:impressumUrl;type:varchar(1024)" json:"impressumUrl"`
	PrivacyPolicyURL  *string `gorm:"column:privacyPolicyUrl;type:varchar(1024)" json:"privacyPolicyUrl"`

	// Federation
	Federation      string         `gorm:"column:federation;type:varchar(128);default:'none'" json:"federation"`
	FederationHosts pq.StringArray `gorm:"column:federationHosts;type:varchar(1024)[];default:'{}'" json:"federationHosts"`

	// Feature flags
	EnableFanoutTimeline           bool `gorm:"column:enableFanoutTimeline;default:true" json:"enableFanoutTimeline"`
	EnableFanoutTimelineDbFallback bool `gorm:"column:enableFanoutTimelineDbFallback;default:true" json:"enableFanoutTimelineDbFallback"`
	ProxyRemoteFiles               bool `gorm:"column:proxyRemoteFiles;default:true" json:"proxyRemoteFiles"`
	SignToActivityPubGet           bool `gorm:"column:signToActivityPubGet;default:true" json:"signToActivityPubGet"`

	// Relations
	RootUser *User `gorm:"foreignKey:RootUserID" json:"rootUser,omitempty"`
}

func (Meta) TableName() string { return "meta" }
