import {
  computed,
  createApp,
  nextTick,
  onBeforeUnmount,
  onMounted,
  reactive,
  ref
} from 'vue';
import { createI18n, useI18n } from 'vue-i18n';
import { Terminal } from 'https://cdn.jsdelivr.net/npm/@xterm/xterm@5.5.0/+esm';
import { FitAddon } from 'https://cdn.jsdelivr.net/npm/@xterm/addon-fit@0.10.0/+esm';

const SUPPORTED_LOCALES = ['en', 'ru'];
const DEFAULT_LOCALE = 'en';
const LOCALE_META = {
  en: {
    label: 'EN',
    nativeLabel: 'English'
  },
  ru: {
    label: 'RU',
    nativeLabel: 'Русский'
  }
};

const SECURITY_ENTRANCE_RESERVED_SEGMENTS = new Set([
  'api',
  'assets',
  'auth',
  'build',
  'favicon.ico',
  'g',
  'index.php',
  'login',
  'p',
  'public',
  'setup',
  'apple-touch-icon.png',
  'safari-pinned-tab.svg'
]);

const inlineFallbackMessages = {
  en: {
    app: {
      eyebrow: 'NOVUS-OS Installer'
    },
    common: {
      back: 'Back',
      continue: 'Continue',
      start: 'Install',
      restart: 'Start again',
      basic: 'Basic',
      advanced: 'Advanced',
      comingSoon: 'Coming soon',
      recommended: 'Recommended',
      enabled: 'Enabled',
      disabled: 'Disabled',
      generate: 'Generate',
      preview: 'Preview',
      optional: 'Optional',
      close: 'Close',
      cancel: 'Cancel',
      save: 'Save',
      disable: 'Disable',
      toggleTheme: 'Toggle theme',
      themeLight: 'Switch to light theme',
      themeDark: 'Switch to dark theme',
      themeModeLight: 'Light',
      themeModeDark: 'Dark'
    },
    wizard: {
      title: 'NOVUS-OS Installation',
      description: 'Configure a new NOVUS installation or restore through the Go installer.',
      steps: {
        welcome: 'Welcome',
        mode: 'Mode',
        domain: 'Domain & SSL',
        admin: 'Admin Profile',
        database: 'Database',
        security: 'Security Perimeter',
        integrations: 'Integrations'
      },
      viewModeTitle: 'Form view mode',
      viewModeBody: 'Use Basic for required fields and enable Advanced only when needed.',
      welcome: {
        title: 'Prepare NOVUS-OS installation',
        body: 'Review installation mode, panel address, administrator account, database settings, key management, and integrations before start.',
        defaults: 'Session',
        heroTitle: 'Current installation settings',
        heroBody: 'Interface language, theme, and selected mode apply to the current session. You can change them before the installation starts.',
        cta: 'Continue',
        restoreCta: 'Switch to restore',
        footer: 'Wizard settings are sent directly to the Go installer and panel services.'
      },
      mode: {
        title: 'Choose installation mode',
        body: 'Select whether this node becomes a clean NOVUS environment or rehydrates from an exported bundle.',
        footer: 'Restore mode accepts a URL, file upload, or pasted encrypted bundle.',
        freshTitle: 'New Install',
        freshBody: 'Prepare a clean panel deployment with databases, agent, and web stack.',
        restoreTitle: 'Restore',
        restoreBody: 'Restore panel configuration from a previously exported bundle.',
        restoreHint: 'Restore settings are applied during installation.'
      },
      restore: {
        liveBadge: 'Live',
        sourceTitle: 'Restore source',
        sourceBody: 'Choose where the encrypted restore envelope comes from. Upload and paste both map to inline payload submission.',
        sourceUrl: 'URL',
        sourceUpload: 'Upload',
        sourcePaste: 'Paste',
        bundleUrl: 'Backup URL',
        bundleUrlPlaceholder: 'https://backup.example.com/novus-export.sgcenc.json',
        bundleUpload: 'Backup file',
        bundleUploadHint: 'Choose a local backup export to embed directly into the setup request.',
        bundlePayload: 'Inline backup payload',
        bundlePayloadPlaceholder: 'Paste the full encrypted NOVUS export envelope here.',
        importModeTitle: 'Import behavior',
        importModeBody: 'Choose whether the restore overwrites existing settings or only fills missing values.',
        importOverwrite: 'Overwrite',
        importSkip: 'Skip existing',
        importReportOnly: 'Report only (no changes)',
        keyTitle: 'Decrypt and unlock',
        keyBody: 'Provide either the raw backup key material or the recovery phrase used to derive it. One of these must be present for restore mode.',
        keyMaterial: 'Backup key material',
        keyMaterialPlaceholder: 'Paste raw restore key material if you exported it directly.',
        recoveryPhrase: 'Recovery phrase',
        recoveryPhrasePlaceholder: 'Paste the operator recovery phrase used for the export.',
        summarySource: 'Selected source',
        summaryMode: 'Import mode',
        footer: 'Restore requests are normalized into inline or URL imports before the installer invokes panel-native import services.'
      },
      domain: {
        title: 'Domain and certificate',
        body: 'Define the external panel URL, SSL path, and optional Security Entrance perimeter before the platform comes online.',
        footer: 'Security Entrance settings are persisted through the same panel service the runtime uses after installation.',
        field: 'Panel URL or domain',
        placeholder: 'https://panel.example.com',
        sslTitle: 'SSL certificate strategy',
        cloudflareToken: 'Cloudflare API Token',
        cloudflarePlaceholder: 'Paste your Cloudflare API token',
        customCertificate: 'Certificate body',
        customCertificatePlaceholder: '-----BEGIN CERTIFICATE-----',
        customPrivateKey: 'Private key',
        customPrivateKeyPlaceholder: '-----BEGIN PRIVATE KEY-----',
        securityEntrance: {
          title: 'Security Entrance',
          body: 'Optionally hide the panel behind a non-obvious ingress path and separate outer listener port with rate-limit controls.',
          path: 'Hidden path token',
          pathPlaceholder: 'gate-control',
          port: 'External listener port',
          portPlaceholder: 'Leave empty to reuse the standard port',
          window: 'Rate-limit window seconds',
          attempts: 'Max attempts',
          block: 'Block duration seconds',
          entryPath: 'External path',
          cookiePath: 'Cookie path',
          disabledState: 'Disabled'
        },
        sslOptions: {
          letsencrypt: {
            title: 'Let\'s Encrypt (Automatic)',
            body: 'Issue and configure the certificate automatically during installation.'
          },
          cloudflare: {
            title: 'Cloudflare SSL',
            body: 'Use DNS validation through Cloudflare with your API token.'
          },
          custom: {
            title: 'Custom certificate',
            body: 'Provide your own certificate and private key for immediate use.'
          }
        }
      },
      admin: {
        title: 'Administrator profile',
        body: 'Create the first administrative identity with a generated username, operator-visible password controls, and explicit confirmation before install.',
        footer: 'Generated passwords can be stored before the installation starts.',
        email: 'Administrator email',
        emailPlaceholder: "admin{'@'}example.com",
        login: 'Generated username',
        password: 'Administrator password',
        passwordPlaceholder: 'Strong password',
        confirmPassword: 'Confirm password',
        confirmPasswordPlaceholder: 'Repeat the administrator password',
        generateAndCopy: 'Generate & copy',
        readinessTitle: 'Readiness check',
        readinessBody: 'Validate the handoff that will be used for the first privileged login and panel bootstrap.',
        confirmationStatus: 'Confirmation',
        passwordsMatch: 'Passwords match',
        passwordsMismatch: 'Passwords do not match yet',
        passwordCopied: 'Administrator password generated and copied to clipboard.',
        strength: {
          length: '12+ chars',
          upper: 'A-Z',
          lower: 'a-z',
          digit: '0-9',
          special: 'Special char'
        }
      },
      database: {
        title: 'Database settings',
        body: 'Set the MariaDB passwords for the system administrator and panel before migrations start.',
        footer: 'The installer prepares separate NOVUS OS, identity, and service desk databases.',
        rootPassword: 'MariaDB administrator password',
        panelPassword: 'Panel database password',
        rootPlaceholder: 'MariaDB administrator password',
        panelPlaceholder: 'Panel database password',
        generateAndCopy: 'Generate & copy',
        passwordCopied: 'Database password generated and copied to clipboard.',
        startInstall: 'Start installation',
        summaryTitle: 'Databases',
        summaryBody: 'These databases are created during installation.',
        osDatabase: 'OS database',
        identityDatabase: 'Identity database',
        serviceDeskDatabase: 'Service desk database'
      },
      security: {
        title: 'Keys and KMS',
        body: 'Choose how to prepare the master key and, if needed, an external KMS.',
        footer: 'Key and KMS settings are applied by the installer during deployment.',
        hybridTitle: 'Automatic key management',
        hybridBody: 'The installer generates the master key and configures the default backend automatically.',
        manualTitle: 'Manual key input',
        manualBody: 'Provide the master key manually in hex or base64.',
        cloudflareTitle: 'Cloudflare KMS',
        cloudflareBody: 'Use a Cloudflare Worker for master-key operations.',
        tpm2Title: 'TPM2',
        tpm2Body: 'Use TPM2 when the target server supports hardware key protection.',
        manualMaterialTitle: 'Manual master key',
        manualMaterialBody: 'Generate a key or paste an existing value.',
        manualGenerate: 'Generate material',
        manualFormat: 'Manual key format',
        manualHex: 'Hex',
        manualBase64: 'Base64',
        keyLabel: 'Master Key',
        keyPlaceholder: 'Paste master-key material',
        cloudflareConfigTitle: 'Cloudflare KMS settings',
        cloudflareConfigBody: 'Provide the credentials and worker endpoint for Cloudflare KMS.',
        cloudflareToken: 'Cloudflare API Token',
        cloudflareTokenPlaceholder: 'Leave empty to reuse the SSL token when applicable',
        cloudflareReuseSsl: 'If left empty, the installer reuses the SSL Cloudflare token when available.',
        cloudflareAccountId: 'Cloudflare account ID',
        cloudflareAccountPlaceholder: 'Cloudflare account identifier',
        cloudflareWorkerUrl: 'Worker URL',
        cloudflareWorkerPlaceholder: 'https://novus-kms.example.workers.dev',
        cloudflareAdvancedTitle: 'Advanced worker settings',
        cloudflareAdvancedBody: 'Specify script, namespace, and route only when required.',
        cloudflareScriptName: 'Worker script name',
        cloudflareNamespaceTitle: 'KV namespace title',
        cloudflareZoneId: 'Zone ID',
        cloudflareZonePlaceholder: 'Optional zone identifier',
        cloudflareRoutePattern: 'Route pattern',
        cloudflareRoutePlaceholder: 'Optional route pattern such as kms.example.com/*',
        summaryTitle: 'Key configuration summary',
        summaryBackend: 'Backend',
        summaryMode: 'Mode',
        summaryAutomatic: 'Automatic generation',
        hint: 'The installer writes environment settings, master-key material, and optional shared secrets in a compatible layout.'
      },
      integrations: {
        title: 'Integrations',
        body: 'Save external providers now if they should be available on first start.',
        footer: 'Only enabled providers with complete required fields are sent in the installer payload.',
        catalogTitle: 'Providers',
        catalogBody: 'Select an integration and add it to the configured list. Click a mini card to edit settings.',
        selectLabel: 'Select integration',
        addSelected: 'Add or configure integration',
        sidebarTitle: 'Configured integrations',
        sidebarEmpty: 'No integrations added yet.',
        quickStatusReady: 'Ready',
        quickStatusIncomplete: 'Needs fields',
        startInstall: 'Start installation',
        configured: 'Configured',
        notConfigured: 'Not configured',
        configuredCount: 'Configured providers',
        fieldSet: 'fields',
        configure: 'Configure',
        add: 'Add',
        modalEyebrow: 'Provider settings',
        enableTitle: 'Enable provider',
        enableBody: 'Only enabled providers are serialized into the setup request.',
        categories: {
          operations: 'Operations',
          identity: 'Identity',
          edge: 'Edge',
          commerce: 'Commerce',
          delivery: 'Delivery'
        },
        invalidProvider: 'Fill all required fields before enabling this provider.',
        providers: {
          telegram: {
            title: 'Telegram Global Bot',
            description: 'Primary global Telegram bot for installer notifications and runtime alerts.',
            fields: {
              token: 'Bot token',
              tokenPlaceholder: 'Telegram bot token',
              adminId: 'Administrator chat ID',
              adminIdPlaceholder: 'Telegram administrator chat ID',
              oauthToken: 'Telegram OAuth2 token',
              oauthTokenPlaceholder: 'Telegram OAuth2 token'
            }
          },
          discordNotifications: {
            title: 'Discord Global Bot',
            description: 'Primary global Discord bot for notifications and operational messages.',
            fields: {
              token: 'Bot token',
              tokenPlaceholder: 'Discord bot token',
              adminId: 'Administrator ID',
              adminIdPlaceholder: 'Discord administrator ID',
              oauthToken: 'Discord OAuth2 token',
              oauthTokenPlaceholder: 'Discord OAuth2 token'
            }
          },
          google: {
            title: 'Google OAuth',
            description: 'Preconfigure Google as an identity provider for operator sign-in.',
            fields: {
              clientId: 'Client ID',
              clientIdPlaceholder: 'Google OAuth client ID',
              clientSecret: 'Client secret',
              clientSecretPlaceholder: 'Google OAuth client secret'
            }
          },
          github: {
            title: 'GitHub OAuth',
            description: 'Preconfigure GitHub sign-in for teams already anchored on GitHub org identity.',
            fields: {
              clientId: 'Client ID',
              clientIdPlaceholder: 'GitHub OAuth client ID',
              clientSecret: 'Client secret',
              clientSecretPlaceholder: 'GitHub OAuth client secret'
            }
          },
          discord: {
            title: 'Discord OAuth',
            description: 'Enable Discord as an identity provider separate from notification transport.',
            fields: {
              clientId: 'Client ID',
              clientIdPlaceholder: 'Discord OAuth client ID',
              clientSecret: 'Client secret',
              clientSecretPlaceholder: 'Discord OAuth client secret'
            }
          },
          cloudflare: {
            title: 'Cloudflare Provider',
            description: 'Persist Cloudflare API credentials for DNS, edge, and worker-driven platform services.',
            fields: {
              apiToken: 'API token',
              apiTokenPlaceholder: 'Cloudflare API token',
              accountId: 'Account ID',
              accountIdPlaceholder: 'Cloudflare account ID'
            }
          },
          steam: {
            title: 'Steam',
            description: 'Store Steam Web API credentials for future catalog or identity workflows.',
            fields: {
              apiKey: 'Steam Web API key',
              apiKeyPlaceholder: 'Steam Web API key'
            }
          },
          smtp: {
            title: 'SMTP Mail Relay',
            description: 'Prime outbound mail with a real SMTP relay instead of leaving panel mail transport for later.',
            fields: {
              host: 'SMTP host',
              hostPlaceholder: 'smtp.example.com',
              port: 'SMTP port',
              portPlaceholder: '587',
              user: 'SMTP user',
              userPlaceholder: 'smtp-user',
              password: 'SMTP password',
              passwordPlaceholder: 'SMTP password',
              fromName: 'From name',
              fromNamePlaceholder: 'NOVUS Control Plane',
              fromEmail: 'From email',
              fromEmailPlaceholder: "noreply{'@'}example.com",
              secure: 'Transport security',
              secureOptions: {
                tls: 'TLS',
                ssl: 'SSL',
                plain: 'Plain'
              }
            }
          }
        }
      },
      summary: {
        mode: 'Mode',
        target: 'Target',
        admin: 'Admin',
        ssl: 'SSL',
        security: 'Security'
      }
    },
    install: {
      eyebrow: 'Installation',
      title: 'NOVUS-OS installation in progress',
      waiting: 'Preparing installation',
      connecting: 'Connecting to the installation session',
      connected: 'Connection established',
      reconnecting: 'Reconnecting in 3 seconds',
      transport: 'Streaming installation steps',
      failed: 'Installation error',
      finished: 'Installation completed',
      target: 'Target',
      admin: 'Admin',
      streamReady: 'Waiting for PTY live stream...'
    },
    success: {
      eyebrow: 'Installation Complete',
      title: 'Installation finished successfully',
      body: 'The panel is installed and the secure sign-in URL is ready.',
      openPanel: 'Open panel'
    },
    errors: {
      setupFailed: 'Unable to start the installation.',
      localeLoad: 'Unable to load interface translations. English will be used instead.',
      restoreDisabled: 'Restore mode requires a valid source and unlock material.',
      restoreFileRead: 'Unable to read the selected restore file.',
      clipboard: 'Unable to copy to clipboard in this browser context.',
      integrationInvalid: 'Fill all required provider fields before saving.',
      securityEntranceInvalid: 'Security Entrance path must be a valid non-reserved token when enabled.'
    }
  }
};

const integrationCatalog = [
  {
    key: 'cloudflare',
    iconKey: 'cloudflare',
    iconBadge: 'CF',
    labelKey: 'wizard.integrations.providers.cloudflare.title',
    descriptionKey: 'wizard.integrations.providers.cloudflare.description',
    categoryKey: 'wizard.integrations.categories.network',
    fields: [
      {
        key: 'cloudflare_api_token',
        labelKey: 'wizard.integrations.providers.cloudflare.fields.apiToken',
        placeholderKey: 'wizard.integrations.providers.cloudflare.fields.apiTokenPlaceholder',
        required: true,
        secret: true,
        type: 'text'
      },
      {
        key: 'cloudflare_account_id',
        labelKey: 'wizard.integrations.providers.cloudflare.fields.accountId',
        placeholderKey: 'wizard.integrations.providers.cloudflare.fields.accountIdPlaceholder',
        required: true,
        type: 'text'
      }
    ]
  },
  {
    key: 'steam',
    iconKey: 'steam',
    iconBadge: 'ST',
    labelKey: 'wizard.integrations.providers.steam.title',
    descriptionKey: 'wizard.integrations.providers.steam.description',
    categoryKey: 'wizard.integrations.categories.commerce',
    fields: [
      {
        key: 'steam_web_api_key',
        labelKey: 'wizard.integrations.providers.steam.fields.apiKey',
        placeholderKey: 'wizard.integrations.providers.steam.fields.apiKeyPlaceholder',
        required: true,
        secret: true,
        type: 'text'
      }
    ]
  },
  {
    key: 'telegram',
    iconKey: 'telegram',
    iconBadge: 'TG',
    labelKey: 'wizard.integrations.providers.telegram.title',
    descriptionKey: 'wizard.integrations.providers.telegram.description',
    categoryKey: 'wizard.integrations.categories.operations',
    fields: [
      {
        key: 'telegram_client_id',
        labelKey: 'wizard.integrations.providers.telegram.fields.clientId',
        placeholderKey: 'wizard.integrations.providers.telegram.fields.clientIdPlaceholder',
        required: false,
        type: 'text'
      },
      {
        key: 'telegram_bot_token',
        labelKey: 'wizard.integrations.providers.telegram.fields.token',
        placeholderKey: 'wizard.integrations.providers.telegram.fields.tokenPlaceholder',
        required: true,
        secret: true,
        type: 'text'
      },
      {
        key: 'telegram_admin_chat_id',
        labelKey: 'wizard.integrations.providers.telegram.fields.adminId',
        placeholderKey: 'wizard.integrations.providers.telegram.fields.adminIdPlaceholder',
        required: true,
        type: 'text'
      },
      {
        key: 'telegram_client_secret',
        labelKey: 'wizard.integrations.providers.telegram.fields.oauthToken',
        placeholderKey: 'wizard.integrations.providers.telegram.fields.oauthTokenPlaceholder',
        required: true,
        secret: true,
        type: 'text'
      }
    ]
  },
  {
    key: 'discord_notifications',
    iconKey: 'discord',
    iconBadge: 'DN',
    labelKey: 'wizard.integrations.providers.discordNotifications.title',
    descriptionKey: 'wizard.integrations.providers.discordNotifications.description',
    categoryKey: 'wizard.integrations.categories.operations',
    fields: [
      {
        key: 'discord_bot_token',
        labelKey: 'wizard.integrations.providers.discordNotifications.fields.token',
        placeholderKey: 'wizard.integrations.providers.discordNotifications.fields.tokenPlaceholder',
        required: true,
        secret: true,
        type: 'text'
      },
      {
        key: 'discord_admin_id',
        labelKey: 'wizard.integrations.providers.discordNotifications.fields.adminId',
        placeholderKey: 'wizard.integrations.providers.discordNotifications.fields.adminIdPlaceholder',
        required: true,
        type: 'text'
      },
      {
        key: 'discord_client_secret',
        labelKey: 'wizard.integrations.providers.discordNotifications.fields.oauthToken',
        placeholderKey: 'wizard.integrations.providers.discordNotifications.fields.oauthTokenPlaceholder',
        required: true,
        secret: true,
        type: 'text'
      }
    ]
  },
  {
    key: 'discord',
    iconKey: 'discord',
    iconBadge: 'DC',
    labelKey: 'wizard.integrations.providers.discord.title',
    descriptionKey: 'wizard.integrations.providers.discord.description',
    categoryKey: 'wizard.integrations.categories.identity',
    fields: [
      {
        key: 'discord_client_id',
        labelKey: 'wizard.integrations.providers.discord.fields.clientId',
        placeholderKey: 'wizard.integrations.providers.discord.fields.clientIdPlaceholder',
        required: true,
        type: 'text'
      },
      {
        key: 'discord_client_secret',
        labelKey: 'wizard.integrations.providers.discord.fields.clientSecret',
        placeholderKey: 'wizard.integrations.providers.discord.fields.clientSecretPlaceholder',
        required: true,
        secret: true,
        type: 'text'
      }
    ]
  },
  {
    key: 'google',
    iconKey: 'google',
    iconBadge: 'G',
    labelKey: 'wizard.integrations.providers.google.title',
    descriptionKey: 'wizard.integrations.providers.google.description',
    categoryKey: 'wizard.integrations.categories.identity',
    fields: [
      {
        key: 'google_client_id',
        labelKey: 'wizard.integrations.providers.google.fields.clientId',
        placeholderKey: 'wizard.integrations.providers.google.fields.clientIdPlaceholder',
        required: true,
        type: 'text'
      },
      {
        key: 'google_client_secret',
        labelKey: 'wizard.integrations.providers.google.fields.clientSecret',
        placeholderKey: 'wizard.integrations.providers.google.fields.clientSecretPlaceholder',
        required: true,
        secret: true,
        type: 'text'
      }
    ]
  },
  {
    key: 'github',
    iconKey: 'github',
    iconBadge: 'GH',
    labelKey: 'wizard.integrations.providers.github.title',
    descriptionKey: 'wizard.integrations.providers.github.description',
    categoryKey: 'wizard.integrations.categories.identity',
    fields: [
      {
        key: 'github_client_id',
        labelKey: 'wizard.integrations.providers.github.fields.clientId',
        placeholderKey: 'wizard.integrations.providers.github.fields.clientIdPlaceholder',
        required: true,
        type: 'text'
      },
      {
        key: 'github_client_secret',
        labelKey: 'wizard.integrations.providers.github.fields.clientSecret',
        placeholderKey: 'wizard.integrations.providers.github.fields.clientSecretPlaceholder',
        required: true,
        secret: true,
        type: 'text'
      }
    ]
  },
  {
    key: 'github_cli',
    iconKey: 'github',
    iconBadge: 'PAT',
    labelKey: 'wizard.integrations.providers.githubCli.title',
    descriptionKey: 'wizard.integrations.providers.githubCli.description',
    categoryKey: 'wizard.integrations.categories.automation',
    fields: [
      {
        key: 'github_pat',
        labelKey: 'wizard.integrations.providers.githubCli.fields.pat',
        placeholderKey: 'wizard.integrations.providers.githubCli.fields.patPlaceholder',
        required: true,
        secret: true,
        type: 'text'
      }
    ]
  },
  {
    key: 'gemini',
    iconKey: 'gemini',
    iconBadge: 'AI',
    labelKey: 'wizard.integrations.providers.gemini.title',
    descriptionKey: 'wizard.integrations.providers.gemini.description',
    categoryKey: 'wizard.integrations.categories.ai',
    fields: [
      {
        key: 'gemini_api_key',
        labelKey: 'wizard.integrations.providers.gemini.fields.apiKey',
        placeholderKey: 'wizard.integrations.providers.gemini.fields.apiKeyPlaceholder',
        required: true,
        secret: true,
        type: 'text'
      }
    ]
  },
  {
    key: 'smtp',
    iconKey: 'mail',
    iconBadge: 'SMTP',
    labelKey: 'wizard.integrations.providers.smtp.title',
    descriptionKey: 'wizard.integrations.providers.smtp.description',
    categoryKey: 'wizard.integrations.categories.delivery',
    fields: [
      {
        key: 'smtp_host',
        labelKey: 'wizard.integrations.providers.smtp.fields.host',
        placeholderKey: 'wizard.integrations.providers.smtp.fields.hostPlaceholder',
        required: true,
        type: 'text'
      },
      {
        key: 'smtp_port',
        labelKey: 'wizard.integrations.providers.smtp.fields.port',
        placeholderKey: 'wizard.integrations.providers.smtp.fields.portPlaceholder',
        required: true,
        type: 'text'
      },
      {
        key: 'smtp_user',
        labelKey: 'wizard.integrations.providers.smtp.fields.user',
        placeholderKey: 'wizard.integrations.providers.smtp.fields.userPlaceholder',
        required: true,
        type: 'text'
      },
      {
        key: 'smtp_password',
        labelKey: 'wizard.integrations.providers.smtp.fields.password',
        placeholderKey: 'wizard.integrations.providers.smtp.fields.passwordPlaceholder',
        required: true,
        secret: true,
        type: 'text'
      },
      {
        key: 'smtp_from_name',
        labelKey: 'wizard.integrations.providers.smtp.fields.fromName',
        placeholderKey: 'wizard.integrations.providers.smtp.fields.fromNamePlaceholder',
        required: true,
        type: 'text'
      },
      {
        key: 'smtp_from_email',
        labelKey: 'wizard.integrations.providers.smtp.fields.fromEmail',
        placeholderKey: 'wizard.integrations.providers.smtp.fields.fromEmailPlaceholder',
        required: true,
        type: 'text'
      },
      {
        key: 'smtp_secure',
        labelKey: 'wizard.integrations.providers.smtp.fields.secure',
        required: false,
        type: 'select',
        defaultValue: 'tls',
        options: [
          {
            value: 'tls',
            labelKey: 'wizard.integrations.providers.smtp.fields.secureOptions.tls'
          },
          {
            value: 'ssl',
            labelKey: 'wizard.integrations.providers.smtp.fields.secureOptions.ssl'
          },
          {
            value: 'plain',
            labelKey: 'wizard.integrations.providers.smtp.fields.secureOptions.plain'
          }
        ]
      }
    ]
  },
  {
    key: 'novus_agent',
    iconKey: 'agent',
    iconBadge: 'AG',
    labelKey: 'wizard.integrations.providers.novusAgent.title',
    descriptionKey: 'wizard.integrations.providers.novusAgent.description',
    categoryKey: 'wizard.integrations.categories.agents',
    fields: [
      {
        key: 'installer_url',
        labelKey: 'wizard.integrations.providers.novusAgent.fields.installerUrl',
        placeholderKey: 'wizard.integrations.providers.novusAgent.fields.installerUrlPlaceholder',
        required: true,
        type: 'text'
      },
      {
        key: 'installer_auth_header',
        labelKey: 'wizard.integrations.providers.novusAgent.fields.authHeader',
        placeholderKey: 'wizard.integrations.providers.novusAgent.fields.authHeaderPlaceholder',
        required: true,
        secret: true,
        type: 'text'
      }
    ]
  }
];

function detectBrowserLocale() {
  const preferred = (navigator.languages && navigator.languages[0]) || navigator.language || DEFAULT_LOCALE;
  const normalized = String(preferred).trim().toLowerCase();
  return normalized.startsWith('ru') ? 'ru' : 'en';
}

async function loadLocaleBundle(lang) {
  const response = await fetch('/api/locales/' + encodeURIComponent(lang), {
    headers: {
      Accept: 'application/json'
    }
  });

  if (!response.ok) {
    throw new Error('locale_fetch_failed:' + lang + ':' + response.status);
  }

  return response.json();
}

function deepMergeMessages(base, override) {
  if (!override || typeof override !== 'object' || Array.isArray(override)) {
    return base;
  }

  const output = Array.isArray(base) ? base.slice() : { ...base };
  Object.keys(override).forEach((key) => {
    const baseValue = output[key];
    const overrideValue = override[key];

    if (
      baseValue
      && overrideValue
      && typeof baseValue === 'object'
      && typeof overrideValue === 'object'
      && !Array.isArray(baseValue)
      && !Array.isArray(overrideValue)
    ) {
      output[key] = deepMergeMessages(baseValue, overrideValue);
      return;
    }

    output[key] = overrideValue;
  });

  return output;
}

async function resolveLocaleBundle(lang, fallback) {
  try {
    const loaded = await loadLocaleBundle(lang);
    return deepMergeMessages(fallback, loaded);
  } catch (_) {
    return fallback;
  }
}

function analyzePassword(value) {
  const input = String(value || '');
  return {
    length: input.length >= 12,
    upper: /[A-Z]/.test(input),
    lower: /[a-z]/.test(input),
    digit: /\d/.test(input),
    special: /[^A-Za-z0-9]/.test(input)
  };
}

function isStrongPassword(value) {
  const rules = analyzePassword(value);
  return Object.values(rules).every(Boolean);
}

function secureRandomInt(max) {
  const random = new Uint32Array(1);
  window.crypto.getRandomValues(random);
  return random[0] % max;
}

function randomBytes(size) {
  const bytes = new Uint8Array(size);
  window.crypto.getRandomValues(bytes);
  return bytes;
}

function generateStrongPassword() {
  const upper = 'ABCDEFGHJKLMNPQRSTUVWXYZ';
  const lower = 'abcdefghijkmnopqrstuvwxyz';
  const digits = '23456789';
  const special = '!@#$%^&*()-_=+[]{}';
  const all = upper + lower + digits + special;
  const seed = [
    upper[secureRandomInt(upper.length)],
    lower[secureRandomInt(lower.length)],
    digits[secureRandomInt(digits.length)],
    special[secureRandomInt(special.length)]
  ];

  while (seed.length < 20) {
    seed.push(all[secureRandomInt(all.length)]);
  }

  for (let index = seed.length - 1; index > 0; index -= 1) {
    const swapIndex = secureRandomInt(index + 1);
    const temp = seed[index];
    seed[index] = seed[swapIndex];
    seed[swapIndex] = temp;
  }

  return seed.join('');
}

function bytesToHex(bytes) {
  return Array.from(bytes, (byte) => byte.toString(16).padStart(2, '0')).join('');
}

function bytesToBase64(bytes) {
  let binary = '';
  bytes.forEach((byte) => {
    binary += String.fromCharCode(byte);
  });
  return window.btoa(binary);
}

function generateManualKey(format) {
  const bytes = randomBytes(32);
  if (format === 'base64') {
    return bytesToBase64(bytes);
  }
  return bytesToHex(bytes);
}

function buildTerminalTheme(isDark) {
  if (isDark) {
    return {
      background: '#020617',
      foreground: '#d4e4ff',
      cursor: '#7dd3fc',
      selectionBackground: 'rgba(56, 189, 248, 0.28)',
      black: '#020617',
      red: '#f87171',
      green: '#4ade80',
      yellow: '#facc15',
      blue: '#60a5fa',
      magenta: '#c084fc',
      cyan: '#22d3ee',
      white: '#e2e8f0',
      brightBlack: '#475569',
      brightRed: '#ff9999',
      brightGreen: '#86efac',
      brightYellow: '#fde047',
      brightBlue: '#93c5fd',
      brightMagenta: '#d8b4fe',
      brightCyan: '#67e8f9',
      brightWhite: '#f8fafc'
    };
  }

  return {
    background: '#f8fafc',
    foreground: '#0f172a',
    cursor: '#0ea5e9',
    selectionBackground: 'rgba(14, 165, 233, 0.20)',
    black: '#0f172a',
    red: '#dc2626',
    green: '#15803d',
    yellow: '#a16207',
    blue: '#2563eb',
    magenta: '#9333ea',
    cyan: '#0f766e',
    white: '#e2e8f0',
    brightBlack: '#64748b',
    brightRed: '#f87171',
    brightGreen: '#4ade80',
    brightYellow: '#eab308',
    brightBlue: '#60a5fa',
    brightMagenta: '#c084fc',
    brightCyan: '#2dd4bf',
    brightWhite: '#111827'
  };
}

function normalizeDomainCandidate(raw) {
  const trimmed = String(raw || '').trim();
  if (!trimmed) {
    return '';
  }
  if (/^[a-z]+:\/\//i.test(trimmed)) {
    return trimmed;
  }
  return 'https://' + trimmed;
}

function deriveLoginFromEmail(value) {
  const input = String(value || '').trim();
  const atIndex = input.indexOf('@');
  if (atIndex <= 0) {
    return '';
  }
  return sanitizeLoginCandidate(input.slice(0, atIndex));
}

function sanitizeLoginCandidate(value) {
  return String(value || '')
    .toLowerCase()
    .replace(/[^a-z0-9._-]/g, '')
    .replace(/^[._-]+/, '')
    .slice(0, 32);
}

function extractHostFromPanelTarget(value) {
  const normalized = normalizeDomainCandidate(value);
  if (!normalized) {
    return '';
  }

  try {
    const parsed = new URL(normalized);
    return String(parsed.hostname || '').trim().toLowerCase();
  } catch (_) {
    return '';
  }
}

function isLikelyIPv4(host) {
  return /^\d{1,3}(?:\.\d{1,3}){3}$/.test(host);
}

function isLikelyIPv6(host) {
  return /:/.test(host);
}

function isLikelyIPAddress(value) {
  const host = extractHostFromPanelTarget(value);
  if (!host) {
    return false;
  }
  return isLikelyIPv4(host) || isLikelyIPv6(host);
}

function trimSlashes(value) {
  return String(value || '').trim().replace(/^\/+|\/+$/g, '');
}

function normalizeRecoveryPhrase(value) {
  return String(value || '').trim().replace(/\s+/g, ' ');
}

function isEmailLike(value) {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(String(value || '').trim());
}

function randomSlug(length) {
  const alphabet = 'abcdefghijkmnopqrstuvwxyz23456789';
  let output = '';
  while (output.length < length) {
    output += alphabet[secureRandomInt(alphabet.length)];
  }
  return output;
}

function fileToText(file) {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onerror = () => reject(reader.error || new Error('file_read_failed'));
    reader.onload = () => resolve(typeof reader.result === 'string' ? reader.result : '');
    reader.readAsText(file);
  });
}

function createDefaultIntegrationState(provider) {
  const fields = {};
  provider.fields.forEach((field) => {
    fields[field.key] = field.defaultValue || '';
  });
  return {
    enabled: false,
    fields
  };
}

function sanitizeOptionalNumber(value) {
  const trimmed = String(value ?? '').trim();
  if (!trimmed) {
    return 0;
  }
  const parsed = Number.parseInt(trimmed, 10);
  return Number.isFinite(parsed) ? parsed : 0;
}

async function bootstrap() {
  const browserLocale = detectBrowserLocale();
  const fallbackMessages = await resolveLocaleBundle(DEFAULT_LOCALE, inlineFallbackMessages.en);
  const messages = {
    [DEFAULT_LOCALE]: fallbackMessages
  };

  for (const lang of SUPPORTED_LOCALES) {
    if (lang === DEFAULT_LOCALE) {
      continue;
    }
    messages[lang] = await resolveLocaleBundle(lang, fallbackMessages);
  }

  const initialLocale = SUPPORTED_LOCALES.includes(browserLocale) ? browserLocale : DEFAULT_LOCALE;

  const i18n = createI18n({
    legacy: false,
    globalInjection: true,
    locale: initialLocale,
    fallbackLocale: DEFAULT_LOCALE,
    messages
  });

  document.documentElement.lang = initialLocale;

  createApp({
    setup() {
      const { t, locale } = useI18n({ useScope: 'global' });
      const mode = ref('wizard');
      const wizardStep = ref(0);
      const submitting = ref(false);
      const errorMessage = ref('');
      const connectionLabelKey = ref('install.waiting');
      const currentStepKind = ref('translation');
      const currentStepValue = ref('install.waiting');
      const statusTone = ref('info');
      const successUrl = ref('');
      const installationFinished = ref(false);
      const terminalEl = ref(null);
      const themeModeResolved = ref('light');
      const restoreInputMode = ref('url');
      const adminLoginTouched = ref(false);
      const toast = reactive({
        message: '',
        kind: 'ok'
      });
      const integrationModal = reactive({
        visible: false,
        providerKey: '',
        enabled: false,
        fields: {}
      });
      const stepAdvancedModes = reactive({
        2: false,
        4: false,
        5: false
      });
      const integrationLibraryOpen = ref(false);
      const integrationPickerKey = ref(integrationCatalog[0] ? integrationCatalog[0].key : '');
      const visibleSecrets = reactive({});
      const localeOptions = SUPPORTED_LOCALES.map((value) => ({
        value,
        label: LOCALE_META[value] ? LOCALE_META[value].label : value.toUpperCase(),
        nativeLabel: LOCALE_META[value] ? LOCALE_META[value].nativeLabel : value.toUpperCase()
      }));
      const wizardKeys = ['welcome', 'mode', 'domain', 'admin', 'database', 'security', 'integrations'];

      const createDefaultForm = () => ({
        installMode: 'new_install',
        panelUrl: '',
        sslMode: 'letsencrypt',
        cloudflareApiToken: '',
        customCertificate: '',
        customPrivateKey: '',
        adminEmail: '',
        adminUsername: '',
        adminPassword: '',
        adminPasswordConfirm: '',
        dbRootPassword: '',
        dbPanelPassword: '',
        githubPat: '',
        manualKeyFormat: 'hex',
        masterKeyMode: 'automatic',
        masterKeyBackend: 'tier1_hybrid_auto_unseal',
        masterKey: '',
        securityEntrance: {
          enabled: false,
          path: '',
          port: 0,
          windowSeconds: 300,
          maxAttempts: 8,
          blockSeconds: 900
        },
        restore: {
          backupUrl: '',
          backupPayload: '',
          keyMaterial: '',
          keyFilePayload: '',
          keyFileName: '',
          keyPassword: '',
          recoveryPhrase: '',
          importMode: 'overwrite',
          localFileName: ''
        },
        cloudflareKms: {
          enabled: false,
          apiToken: '',
          accountId: '',
          scriptName: 'novus-kms-worker',
          namespaceTitle: 'NOVUS_KMS_KEYS',
          zoneId: '',
          routePattern: '',
          workerUrl: ''
        },
        integrations: Object.fromEntries(
          integrationCatalog.map((provider) => [provider.key, createDefaultIntegrationState(provider)])
        )
      });

      const form = reactive(createDefaultForm());
      const activeLocale = computed(() => String(locale.value || DEFAULT_LOCALE));
      const localeLabel = computed(() => {
        const matched = localeOptions.find((entry) => entry.value === activeLocale.value);
        return matched ? matched.label : activeLocale.value.toUpperCase();
      });
      const themeModeLabel = computed(() => (themeModeResolved.value === 'dark' ? t('common.themeModeDark') : t('common.themeModeLight')));
      const suggestedAdminUsername = computed(() => deriveLoginFromEmail(form.adminEmail));
      const adminUsername = computed(() => {
        const manual = sanitizeLoginCandidate(form.adminUsername);
        if (manual) {
          return manual;
        }
        return suggestedAdminUsername.value || 'admin';
      });
      const activeStepKey = computed(() => wizardKeys[wizardStep.value] || 'welcome');
      const totalWizardSteps = computed(() => wizardKeys.length - 1);
      const currentWizardOrdinal = computed(() => Math.max(1, Math.min(totalWizardSteps.value, wizardStep.value)));
      const visibleWizardIndex = computed(() => wizardStep.value === 0 ? -1 : wizardStep.value - 1);
      const panelTargetPreview = computed(() => normalizeDomainCandidate(form.panelUrl));
      const panelTargetDisplay = computed(() => panelTargetPreview.value || t('wizard.summary.targetPending'));
      const adminDisplay = computed(() => String(form.adminEmail || '').trim() || t('wizard.summary.adminPending'));
      const panelTargetUsesIP = computed(() => isLikelyIPAddress(panelTargetPreview.value));
      const securityEntranceSuggested = computed(() => panelTargetUsesIP.value && !form.securityEntrance.enabled);
      const modeLabel = computed(() => (form.installMode === 'restore' ? t('wizard.mode.restoreTitle') : t('wizard.mode.freshTitle')));
      const sslSummaryLabel = computed(() => t('wizard.domain.sslOptions.' + form.sslMode + '.title'));
      const wizardTimeline = computed(() => wizardKeys.map((key) => ({
        key,
        label: t('wizard.steps.' + key)
      })));
      const visibleWizardTimeline = computed(() => wizardTimeline.value.filter((step) => step.key !== 'welcome'));
      const isFinalWizardStep = computed(() => wizardStep.value === wizardKeys.length - 1);
      const adminPasswordMatches = computed(() => String(form.adminPassword).length > 0 && form.adminPassword === form.adminPasswordConfirm);
      const adminPasswordRules = computed(() => {
        const rules = analyzePassword(form.adminPassword);
        return [
          { key: 'length', label: t('wizard.admin.strength.length'), ok: rules.length },
          { key: 'upper', label: t('wizard.admin.strength.upper'), ok: rules.upper },
          { key: 'lower', label: t('wizard.admin.strength.lower'), ok: rules.lower },
          { key: 'digit', label: t('wizard.admin.strength.digit'), ok: rules.digit },
          { key: 'special', label: t('wizard.admin.strength.special'), ok: rules.special }
        ];
      });
      const securityPlan = computed(() => {
        if (form.masterKeyMode === 'manual' || form.masterKeyMode === 'import') {
          return 'manual';
        }
        if (form.masterKeyBackend === 'tier2_cloudflare_zero_disk_kms') {
          return 'cloudflare';
        }
        if (form.masterKeyBackend === 'tier3_tpm2_hardware_sealing') {
          return 'tpm2';
        }
        return 'automatic';
      });
      const securityPlanLabel = computed(() => {
        switch (securityPlan.value) {
          case 'manual':
            return t('wizard.security.manualTitle');
          case 'cloudflare':
            return t('wizard.security.cloudflareTitle');
          case 'tpm2':
            return t('wizard.security.tpm2Title');
          default:
            return t('wizard.security.hybridTitle');
        }
      });
      const masterKeyModeLabel = computed(() => (form.masterKeyMode === 'manual' ? t('wizard.security.manualTitle') : t('wizard.security.summaryAutomatic')));
      const restoreSummaryLabel = computed(() => {
        if (restoreInputMode.value === 'url') {
          return String(form.restore.backupUrl || '').trim() || t('wizard.restore.sourceUrl');
        }
        if (restoreInputMode.value === 'upload') {
          return form.restore.localFileName || t('wizard.restore.sourceUpload');
        }
        return t('wizard.restore.sourcePaste');
      });
      const securityEntranceEntryPath = computed(() => {
        if (!form.securityEntrance.enabled) {
          return t('wizard.domain.securityEntrance.disabledState');
        }
        const token = trimSlashes(form.securityEntrance.path);
        return token ? '/' + token : '/';
      });
      const securityEntranceCookiePath = computed(() => {
        if (!form.securityEntrance.enabled) {
          return '/';
        }
        const token = trimSlashes(form.securityEntrance.path);
        return token ? '/' + token + '/' : '/';
      });
      const configuredIntegrationCount = computed(() => integrationCatalog.filter((provider) => form.integrations[provider.key] && form.integrations[provider.key].enabled).length);
      const selectedIntegrationProviders = computed(() => integrationCatalog.filter((provider) => form.integrations[provider.key] && form.integrations[provider.key].enabled));
      const currentIntegrationProvider = computed(() => integrationCatalog.find((provider) => provider.key === integrationModal.providerKey) || null);
      const connectionLabel = computed(() => t(connectionLabelKey.value));
      const currentStep = computed(() => (currentStepKind.value === 'translation' ? t(currentStepValue.value) : currentStepValue.value));
      const statusBadgeClass = computed(() => {
        if (statusTone.value === 'success') {
          return 'status-chip status-chip--success';
        }
        if (statusTone.value === 'error') {
          return 'status-chip status-chip--error';
        }
        if (statusTone.value === 'warning') {
          return 'status-chip status-chip--warning';
        }
        return 'status-chip status-chip--info';
      });
      const footerNote = computed(() => {
        return '';
      });
      const primaryActionLabel = computed(() => {
        if (isFinalWizardStep.value) {
          return submitting.value ? t('install.waiting') : t('wizard.integrations.startInstall');
        }
        return t('common.continue');
      });

      const securityEntranceValid = () => {
        if (!form.securityEntrance.enabled) {
          return true;
        }

        const token = trimSlashes(form.securityEntrance.path);
        if (!token || !/^[a-z0-9](?:[a-z0-9_-]{1,62}[a-z0-9])?$/.test(token) || SECURITY_ENTRANCE_RESERVED_SEGMENTS.has(token)) {
          return false;
        }

        if (form.securityEntrance.port) {
          const port = sanitizeOptionalNumber(form.securityEntrance.port);
          if (port < 1 || port > 65535) {
            return false;
          }
        }

        return true;
      };

      const restoreSourceReady = () => {
        if (restoreInputMode.value === 'url') {
          return String(form.restore.backupUrl || '').trim().length > 0;
        }
        return String(form.restore.backupPayload || '').trim().length > 0;
      };

      const looksLikeEncryptedEnvelope = (value) => {
        const raw = String(value || '').trim();
        if (!raw || raw[0] !== '{') {
          return false;
        }

        try {
          const parsed = JSON.parse(raw);
          if (!parsed || typeof parsed !== 'object') {
            return false;
          }

          const hasVersion = parsed.v === 1 || parsed.v === '1';
          const hasCipherFields = ['salt', 'iv', 'tag', 'ct'].every((field) => typeof parsed[field] === 'string' && parsed[field].trim().length > 0);
          return hasVersion && hasCipherFields;
        } catch (_error) {
          return false;
        }
      };

      const resolveRestoreUnlockMaterial = () => {
        const directMaterial = String(form.restore.keyMaterial || '').trim();
        if (directMaterial) {
          return directMaterial;
        }

        const uploadedKey = String(form.restore.keyFilePayload || '').trim();
        const uploadedKeyPassword = String(form.restore.keyPassword || '').trim();
        if (uploadedKey) {
          if (uploadedKeyPassword && looksLikeEncryptedEnvelope(uploadedKey)) {
            return uploadedKeyPassword;
          }
          return uploadedKey;
        }
        if (uploadedKeyPassword) {
          return uploadedKeyPassword;
        }

        return normalizeRecoveryPhrase(form.restore.recoveryPhrase);
      };

      const restoreUnlockReady = () => resolveRestoreUnlockMaterial().length > 0;

      const validateIntegrationState = (provider, state) => {
        if (!state || !state.enabled) {
          return true;
        }
        return provider.fields.every((field) => {
          if (!field.required) {
            return true;
          }
          return String(state.fields[field.key] || '').trim().length > 0;
        });
      };

      const integrationsValid = () => integrationCatalog.every((provider) => validateIntegrationState(provider, form.integrations[provider.key]));

      const canContinue = computed(() => {
        if (wizardStep.value === 0) {
          return true;
        }
        if (wizardStep.value === 1) {
          if (form.installMode !== 'restore') {
            return true;
          }
          return restoreSourceReady() && restoreUnlockReady();
        }
        if (wizardStep.value === 2) {
          if (String(panelTargetPreview.value || '').trim().length === 0) {
            return false;
          }
          if (form.sslMode === 'cloudflare' && String(form.cloudflareApiToken || '').trim().length === 0) {
            return false;
          }
          if (form.sslMode === 'custom') {
            if (String(form.customCertificate || '').trim().length === 0 || String(form.customPrivateKey || '').trim().length === 0) {
              return false;
            }
          }
          return securityEntranceValid();
        }
        if (wizardStep.value === 3) {
          return isEmailLike(form.adminEmail)
            && String(adminUsername.value || '').trim().length >= 3
            && isStrongPassword(form.adminPassword)
            && adminPasswordMatches.value;
        }
        if (wizardStep.value === 4) {
          return isStrongPassword(form.dbRootPassword) && isStrongPassword(form.dbPanelPassword);
        }
        if (wizardStep.value === 5) {
          if (securityPlan.value === 'manual') {
            return String(form.masterKey || '').trim().length > 0;
          }
          if (securityPlan.value === 'cloudflare') {
            return String(form.cloudflareKms.accountId || '').trim().length > 0
              && String(form.cloudflareKms.workerUrl || '').trim().length > 0
              && (String(form.cloudflareKms.apiToken || '').trim().length > 0 || String(form.cloudflareApiToken || '').trim().length > 0);
          }
          return true;
        }
        if (wizardStep.value === 6) {
          return integrationsValid();
        }
        return false;
      });

      let term = null;
      let fitAddon = null;
      let ws = null;
      let resizeHandler = null;
      let reconnectTimer = null;
      let colorSchemeMedia = null;
      let colorSchemeHandler = null;
      let themeManualOverride = false;
      let toastTimer = null;

      const clearToast = () => {
        if (toastTimer) {
          window.clearTimeout(toastTimer);
          toastTimer = null;
        }
      };

      const showToast = (message, kind) => {
        toast.message = message;
        toast.kind = kind || 'ok';
        clearToast();
        toastTimer = window.setTimeout(() => {
          toast.message = '';
          toast.kind = 'ok';
          toastTimer = null;
        }, 2800);
      };

      const clearReconnectTimer = () => {
        if (reconnectTimer) {
          window.clearTimeout(reconnectTimer);
          reconnectTimer = null;
        }
      };

      const applyResolvedTheme = (modeName) => {
        const isDark = modeName === 'dark';
        themeModeResolved.value = isDark ? 'dark' : 'light';
        document.documentElement.classList.toggle('dark', isDark);
        document.documentElement.style.colorScheme = isDark ? 'dark' : 'light';
        if (term) {
          term.options.theme = buildTerminalTheme(isDark);
        }
      };

      const applySystemTheme = (isDark) => {
        if (themeManualOverride) {
          return;
        }
        applyResolvedTheme(isDark ? 'dark' : 'light');
      };

      const toggleTheme = () => {
        themeManualOverride = true;
        applyResolvedTheme(themeModeResolved.value === 'dark' ? 'light' : 'dark');
      };

      const fitTerminal = () => {
        if (!fitAddon) {
          return;
        }
        requestAnimationFrame(() => {
          try {
            fitAddon.fit();
          } catch (_) {
            // Ignore transient fit failures while the layout animates.
          }
        });
      };

      const disconnectStream = () => {
        clearReconnectTimer();
        if (!ws) {
          return;
        }

        const socket = ws;
        ws = null;
        socket.onopen = null;
        socket.onmessage = null;
        socket.onerror = null;
        socket.onclose = null;
        socket.close();
      };

      const disposeTerminal = () => {
        if (resizeHandler) {
          window.removeEventListener('resize', resizeHandler);
          resizeHandler = null;
        }
        disconnectStream();
        if (term) {
          term.dispose();
          term = null;
        }
        fitAddon = null;
      };

      const scheduleReconnect = () => {
        if (installationFinished.value || mode.value !== 'installing' || reconnectTimer) {
          return;
        }

        statusTone.value = 'warning';
        connectionLabelKey.value = 'install.reconnecting';
        reconnectTimer = window.setTimeout(() => {
          reconnectTimer = null;
          connectStream();
        }, 3000);
      };

      const handleStatusFrame = (raw) => {
        let payload = null;
        try {
          payload = JSON.parse(raw);
        } catch (_) {
          if (term) {
            term.write(raw);
          }
          return;
        }

        const text = typeof payload?.text === 'string' ? payload.text : '';
        if (payload?.type === 'step') {
          currentStepKind.value = text ? 'raw' : 'translation';
          currentStepValue.value = text || 'install.waiting';
          connectionLabelKey.value = 'install.transport';
          statusTone.value = 'info';
          if (errorMessage.value) {
            errorMessage.value = '';
          }
          return;
        }

        if (payload?.type === 'error') {
          currentStepKind.value = text ? 'raw' : 'translation';
          currentStepValue.value = text || 'install.failed';
          connectionLabelKey.value = 'install.failed';
          statusTone.value = 'error';
          errorMessage.value = text || t('errors.setupFailed');
          return;
        }

        if (payload?.type === 'finish') {
          installationFinished.value = true;
          currentStepKind.value = text ? 'raw' : 'translation';
          currentStepValue.value = text || 'install.finished';
          connectionLabelKey.value = 'install.finished';
          statusTone.value = 'success';
          successUrl.value = typeof payload?.url === 'string' && payload.url ? payload.url : panelTargetPreview.value;
          mode.value = 'success';
          disconnectStream();
        }
      };

      const connectStream = () => {
        if (!term) {
          return;
        }
        if (ws && (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING)) {
          return;
        }

        clearReconnectTimer();
        const protocol = window.location.protocol === 'https:' ? 'wss://' : 'ws://';
        ws = new WebSocket(protocol + window.location.host + '/api/stream');
        ws.binaryType = 'arraybuffer';

        ws.onopen = () => {
          connectionLabelKey.value = 'install.connected';
          if (!installationFinished.value && statusTone.value !== 'error') {
            statusTone.value = 'info';
          }
          fitTerminal();
        };

        ws.onmessage = (event) => {
          if (typeof event.data === 'string') {
            handleStatusFrame(event.data);
            return;
          }
          if (event.data instanceof ArrayBuffer && term) {
            term.write(new Uint8Array(event.data));
          }
        };

        ws.onerror = () => {
          if (term) {
            term.write('\r\n\x1b[31m[frontend]\x1b[0m websocket error\r\n');
          }
          if (!installationFinished.value) {
            connectionLabelKey.value = 'install.failed';
            statusTone.value = 'warning';
          }
        };

        ws.onclose = () => {
          ws = null;
          if (installationFinished.value) {
            return;
          }
          if (term) {
            term.write('\r\n\x1b[33m[frontend]\x1b[0m websocket connection closed, reconnecting in 3s...\r\n');
          }
          scheduleReconnect();
        };
      };

      const openTerminal = async () => {
        for (let attempt = 0; attempt < 24 && !terminalEl.value; attempt += 1) {
          await nextTick();
          if (terminalEl.value) {
            break;
          }
          await new Promise((resolve) => {
            requestAnimationFrame(() => resolve());
          });
        }

        if (!terminalEl.value) {
          throw new Error('terminal_mount_missing');
        }

        if (term) {
          fitTerminal();
          connectStream();
          return;
        }

        fitAddon = new FitAddon();
        term = new Terminal({
          cursorBlink: true,
          convertEol: false,
          fontFamily: 'JetBrains Mono, ui-monospace, monospace',
          fontSize: 14,
          lineHeight: 1.25,
          letterSpacing: 0.15,
          scrollback: 10000,
          theme: buildTerminalTheme(themeModeResolved.value === 'dark')
        });

        term.loadAddon(fitAddon);
        term.open(terminalEl.value);
        fitTerminal();
        term.write('\x1b[1;36mNOVUS-OS Installer\x1b[0m\r\n');
        term.write('\x1b[2m' + t('install.streamReady') + '\x1b[0m\r\n\r\n');

        resizeHandler = () => fitTerminal();
        window.addEventListener('resize', resizeHandler);

        connectStream();
      };

      const switchLocale = (nextLocale, sourceEvent) => {
        if (!SUPPORTED_LOCALES.includes(nextLocale)) {
          return;
        }

        if (sourceEvent && sourceEvent.currentTarget) {
          const picker = sourceEvent.currentTarget.closest('.locale-picker');
          if (picker) {
            picker.removeAttribute('open');
          }
        }

        if (locale.value === nextLocale) {
          return;
        }

        locale.value = nextLocale;
        document.documentElement.lang = nextLocale;
      };

      const copyToClipboard = async (text) => {
        if (navigator.clipboard && typeof navigator.clipboard.writeText === 'function') {
          await navigator.clipboard.writeText(text);
          return;
        }

        const textarea = document.createElement('textarea');
        textarea.value = text;
        textarea.setAttribute('readonly', 'readonly');
        textarea.style.position = 'absolute';
        textarea.style.left = '-9999px';
        document.body.appendChild(textarea);
        textarea.select();
        const copied = document.execCommand('copy');
        document.body.removeChild(textarea);
        if (!copied) {
          throw new Error('clipboard_failed');
        }
      };

      const toggleVisibility = (key) => {
        visibleSecrets[key] = !visibleSecrets[key];
      };

      const isVisible = (key) => Boolean(visibleSecrets[key]);

      const generatePassword = async (field, mirrorConfirm) => {
        const password = generateStrongPassword();
        form[field] = password;
        if (mirrorConfirm) {
          form.adminPasswordConfirm = password;
        }
        try {
          await copyToClipboard(password);
          showToast(field === 'adminPassword' ? t('wizard.admin.passwordCopied') : t('wizard.database.passwordCopied'), 'ok');
        } catch (_) {
          showToast(t('errors.clipboard'), 'error');
        }
      };

      const generateManualMasterKey = async () => {
        form.masterKey = generateManualKey(form.manualKeyFormat);
        try {
          await copyToClipboard(form.masterKey);
          showToast(t('wizard.security.manualGenerate') + ': ' + t('common.enabled'), 'ok');
        } catch (_) {
          showToast(t('errors.clipboard'), 'error');
        }
      };

      const handleAdminEmailInput = (event) => {
        const nextValue = event && event.target ? String(event.target.value || '') : String(form.adminEmail || '');
        form.adminEmail = nextValue;
        if (!adminLoginTouched.value) {
          form.adminUsername = suggestedAdminUsername.value;
        }
      };

      const handleAdminLoginInput = (event) => {
        const nextValue = event && event.target ? String(event.target.value || '') : String(form.adminUsername || '');
        const sanitized = sanitizeLoginCandidate(nextValue);
        form.adminUsername = sanitized;
        adminLoginTouched.value = sanitized.length > 0;
      };

      const normalizeDomainInput = () => {
        form.panelUrl = normalizeDomainCandidate(form.panelUrl);
      };

      const selectInstallMode = (nextMode) => {
        form.installMode = nextMode;
        errorMessage.value = '';
      };

      const startRestoreWizard = () => {
        selectInstallMode('restore');
        wizardStep.value = 1;
      };

      const selectSSLMode = (nextMode) => {
        form.sslMode = nextMode;
        if (nextMode !== 'cloudflare') {
          form.cloudflareApiToken = '';
        }
        if (nextMode !== 'custom') {
          form.customCertificate = '';
          form.customPrivateKey = '';
        }
        errorMessage.value = '';
      };

      const selectSecurityPlan = (plan) => {
        switch (plan) {
          case 'manual':
            form.masterKeyMode = 'manual';
            form.masterKeyBackend = 'tier1_hybrid_auto_unseal';
            form.cloudflareKms.enabled = false;
            break;
          case 'cloudflare':
            form.masterKeyMode = 'automatic';
            form.masterKeyBackend = 'tier2_cloudflare_zero_disk_kms';
            form.cloudflareKms.enabled = true;
            if (!form.cloudflareKms.apiToken && form.cloudflareApiToken) {
              form.cloudflareKms.apiToken = form.cloudflareApiToken;
            }
            break;
          case 'tpm2':
            form.masterKeyMode = 'automatic';
            form.masterKeyBackend = 'tier3_tpm2_hardware_sealing';
            form.cloudflareKms.enabled = false;
            break;
          default:
            form.masterKeyMode = 'automatic';
            form.masterKeyBackend = 'tier1_hybrid_auto_unseal';
            form.cloudflareKms.enabled = false;
            break;
        }
        errorMessage.value = '';
      };

      const setRestoreInputMode = (nextMode) => {
        restoreInputMode.value = nextMode;
        if (nextMode === 'url') {
          form.restore.localFileName = '';
        } else {
          form.restore.backupUrl = '';
        }
      };

      const handleRestoreFileSelection = async (event) => {
        const target = event.target;
        const file = target && target.files && target.files[0] ? target.files[0] : null;
        if (!file) {
          return;
        }
        try {
          form.restore.backupPayload = await fileToText(file);
          form.restore.localFileName = file.name;
          restoreInputMode.value = 'upload';
          showToast(file.name, 'ok');
        } catch (_) {
          showToast(t('errors.restoreFileRead'), 'error');
        }
      };

      const handleRestoreKeyFileSelection = async (event) => {
        const target = event.target;
        const file = target && target.files && target.files[0] ? target.files[0] : null;
        if (!file) {
          return;
        }

        try {
          form.restore.keyFilePayload = await fileToText(file);
          form.restore.keyFileName = file.name;
          showToast(file.name, 'ok');
        } catch (_) {
          showToast(t('errors.restoreFileRead'), 'error');
        }
      };

      const handleSecurityEntranceToggle = () => {
        if (form.securityEntrance.enabled && !trimSlashes(form.securityEntrance.path)) {
          form.securityEntrance.path = 'gate-' + randomSlug(8);
        }
        errorMessage.value = '';
      };

      const suggestSecurityEntranceForIp = () => {
        if (!panelTargetUsesIP.value) {
          return;
        }
        form.securityEntrance.enabled = true;
        handleSecurityEntranceToggle();
      };

      const nextWizardStep = () => {
        if (!canContinue.value) {
          if (wizardStep.value === 2 && !securityEntranceValid()) {
            errorMessage.value = t('errors.securityEntranceInvalid');
          } else if (wizardStep.value === 1 && form.installMode === 'restore') {
            errorMessage.value = t('errors.restoreDisabled');
          }
          return;
        }
        errorMessage.value = '';
        integrationLibraryOpen.value = false;
        wizardStep.value = Math.min(wizardKeys.length - 1, wizardStep.value + 1);
      };

      const previousWizardStep = () => {
        errorMessage.value = '';
        integrationLibraryOpen.value = false;
        wizardStep.value = Math.max(0, wizardStep.value - 1);
      };

      const isIntegrationConfigured = (key) => Boolean(form.integrations[key] && form.integrations[key].enabled);

      const isStepAdvanced = (stepNumber) => Boolean(stepAdvancedModes[stepNumber]);

      const setStepDetailMode = (stepNumber, nextMode) => {
        stepAdvancedModes[stepNumber] = nextMode === 'advanced';
      };

      const isIntegrationStateValid = (provider) => validateIntegrationState(provider, form.integrations[provider.key]);

      const openIntegrationModal = (providerKey, options = {}) => {
        const provider = integrationCatalog.find((item) => item.key === providerKey);
        if (!provider) {
          return;
        }
        const state = form.integrations[provider.key] || createDefaultIntegrationState(provider);
        integrationModal.visible = true;
        integrationModal.providerKey = provider.key;
        integrationModal.enabled = options.forceEnable ? true : Boolean(state.enabled);
        integrationModal.fields = { ...state.fields };
        provider.fields.forEach((field) => {
          if (typeof integrationModal.fields[field.key] !== 'string') {
            integrationModal.fields[field.key] = field.defaultValue || '';
          }
        });
      };

      const openPickedIntegrationModal = () => {
        if (!integrationPickerKey.value) {
          return;
        }
        openIntegrationModal(integrationPickerKey.value, { forceEnable: true });
      };

      const openIntegrationLibrary = () => {
        integrationLibraryOpen.value = true;
      };

      const closeIntegrationLibrary = () => {
        integrationLibraryOpen.value = false;
      };

      const openIntegrationFromLibrary = (providerKey) => {
        openIntegrationModal(providerKey, { forceEnable: true });
      };

      const closeIntegrationModal = () => {
        integrationModal.visible = false;
        integrationModal.providerKey = '';
        integrationModal.enabled = false;
        integrationModal.fields = {};
      };

      const disableCurrentIntegration = () => {
        const provider = currentIntegrationProvider.value;
        if (!provider) {
          return;
        }
        form.integrations[provider.key] = createDefaultIntegrationState(provider);
        closeIntegrationModal();
      };

      const saveIntegrationModal = () => {
        const provider = currentIntegrationProvider.value;
        if (!provider) {
          return;
        }

        const nextState = {
          enabled: Boolean(integrationModal.enabled),
          fields: {}
        };

        provider.fields.forEach((field) => {
          nextState.fields[field.key] = String(integrationModal.fields[field.key] || field.defaultValue || '').trim();
        });

        if (nextState.enabled && !validateIntegrationState(provider, nextState)) {
          showToast(t('wizard.integrations.invalidProvider'), 'error');
          return;
        }

        form.integrations[provider.key] = nextState;
        closeIntegrationModal();
      };

      const buildIntegrationsPayload = () => integrationCatalog
        .filter((provider) => form.integrations[provider.key] && form.integrations[provider.key].enabled)
        .map((provider) => {
          const state = form.integrations[provider.key];
          const fields = {};
          provider.fields.forEach((field) => {
            const value = String(state.fields[field.key] || '').trim();
            if (value) {
              fields[field.key] = value;
            }
          });
          return {
            Key: provider.key,
            Enabled: true,
            Fields: fields
          };
        });

      const buildSetupPayload = () => {
        const integrations = buildIntegrationsPayload();
        const telegram = form.integrations.telegram;
        const discordNotifications = form.integrations.discord_notifications;

        return {
          Domain: panelTargetPreview.value,
          InstallMode: form.installMode,
          UseLetsEncrypt: form.sslMode === 'letsencrypt',
          SSLMode: form.sslMode,
          CloudflareAPIToken: form.cloudflareApiToken,
          CustomCertificate: form.customCertificate,
          CustomPrivateKey: form.customPrivateKey,
          AdminEmail: form.adminEmail,
          AdminUsername: adminUsername.value,
          AdminPassword: form.adminPassword,
          DBRootPassword: form.dbRootPassword,
          DBPanelPassword: form.dbPanelPassword,
          GitHubPAT: String(form.githubPat || '').trim() || (typeof NOVUS_INSTALLER_GITHUB_PAT !== 'undefined' ? String(NOVUS_INSTALLER_GITHUB_PAT).trim() : ''),
          MasterKeyMode: form.masterKeyMode,
          MasterKey: form.masterKey,
          MasterKeyBackend: form.masterKeyBackend,
          SecurityEntrance: {
            Enabled: Boolean(form.securityEntrance.enabled),
            Path: trimSlashes(form.securityEntrance.path),
            Port: sanitizeOptionalNumber(form.securityEntrance.port),
            WindowSeconds: sanitizeOptionalNumber(form.securityEntrance.windowSeconds) || 300,
            MaxAttempts: sanitizeOptionalNumber(form.securityEntrance.maxAttempts) || 8,
            BlockSeconds: sanitizeOptionalNumber(form.securityEntrance.blockSeconds) || 900
          },
          Restore: {
            SourceType: restoreInputMode.value === 'url' ? 'url' : 'inline',
            BackupURL: restoreInputMode.value === 'url' ? form.restore.backupUrl : '',
            BackupFile: '',
            BackupPayload: restoreInputMode.value === 'url' ? '' : form.restore.backupPayload,
            KeyMaterial: resolveRestoreUnlockMaterial(),
            RecoveryPhrase: normalizeRecoveryPhrase(form.restore.recoveryPhrase),
            ImportMode: form.restore.importMode
          },
          CloudflareKMS: {
            Enabled: securityPlan.value === 'cloudflare',
            APIToken: form.cloudflareKms.apiToken,
            AccountID: form.cloudflareKms.accountId,
            ScriptName: form.cloudflareKms.scriptName,
            NamespaceTitle: form.cloudflareKms.namespaceTitle,
            ZoneID: form.cloudflareKms.zoneId,
            RoutePattern: form.cloudflareKms.routePattern,
            WorkerURL: form.cloudflareKms.workerUrl
          },
          TelegramEnabled: Boolean(telegram && telegram.enabled),
          TelegramBotToken: telegram ? String(telegram.fields.telegram_bot_token || '') : '',
          TelegramAdminID: telegram ? String(telegram.fields.telegram_admin_chat_id || '') : '',
          DiscordEnabled: Boolean(discordNotifications && discordNotifications.enabled),
          DiscordBotToken: discordNotifications ? String(discordNotifications.fields.discord_bot_token || '') : '',
          DiscordAdminID: discordNotifications ? String(discordNotifications.fields.discord_admin_id || '') : '',
          Integrations: integrations
        };
      };

      const advanceWizard = () => {
        if (isFinalWizardStep.value) {
          void submitSetup();
          return;
        }
        nextWizardStep();
      };

      const submitSetup = async () => {
        if (submitting.value || !canContinue.value) {
          return;
        }

        submitting.value = true;
        errorMessage.value = '';
        integrationLibraryOpen.value = false;
        normalizeDomainInput();

        try {
          const response = await fetch('/api/setup', {
            method: 'POST',
            headers: {
              'Content-Type': 'application/json'
            },
            body: JSON.stringify(buildSetupPayload())
          });

          let payload = null;
          try {
            payload = await response.json();
          } catch (_) {
            payload = null;
          }

          if (!response.ok || payload?.status !== 'installing') {
            throw new Error(payload?.error || payload?.status || t('errors.setupFailed'));
          }

          installationFinished.value = false;
          mode.value = 'installing';
          currentStepKind.value = 'translation';
          currentStepValue.value = 'install.waiting';
          connectionLabelKey.value = 'install.connecting';
          statusTone.value = 'info';
          await openTerminal();
        } catch (error) {
          errorMessage.value = error instanceof Error ? error.message : t('errors.setupFailed');
          mode.value = 'wizard';
        } finally {
          submitting.value = false;
        }
      };

      const restartInstaller = () => {
        disposeTerminal();
        installationFinished.value = false;
        errorMessage.value = '';
        connectionLabelKey.value = 'install.waiting';
        currentStepKind.value = 'translation';
        currentStepValue.value = 'install.waiting';
        statusTone.value = 'info';
        successUrl.value = '';
        Object.assign(form, createDefaultForm());
        adminLoginTouched.value = false;
        restoreInputMode.value = 'url';
        closeIntegrationModal();
        Object.keys(visibleSecrets).forEach((key) => {
          delete visibleSecrets[key];
        });
        stepAdvancedModes[2] = false;
        stepAdvancedModes[4] = false;
        stepAdvancedModes[5] = false;
        integrationLibraryOpen.value = false;
        integrationPickerKey.value = integrationCatalog[0] ? integrationCatalog[0].key : '';
        mode.value = 'wizard';
        wizardStep.value = 0;
      };

      onMounted(() => {
        colorSchemeMedia = window.matchMedia('(prefers-color-scheme: dark)');
        colorSchemeHandler = (event) => applySystemTheme(event.matches);
        applyResolvedTheme(colorSchemeMedia.matches ? 'dark' : 'light');

        if (typeof colorSchemeMedia.addEventListener === 'function') {
          colorSchemeMedia.addEventListener('change', colorSchemeHandler);
        } else if (typeof colorSchemeMedia.addListener === 'function') {
          colorSchemeMedia.addListener(colorSchemeHandler);
        }
      });

      onBeforeUnmount(() => {
        clearToast();
        if (colorSchemeMedia && colorSchemeHandler) {
          if (typeof colorSchemeMedia.removeEventListener === 'function') {
            colorSchemeMedia.removeEventListener('change', colorSchemeHandler);
          } else if (typeof colorSchemeMedia.removeListener === 'function') {
            colorSchemeMedia.removeListener(colorSchemeHandler);
          }
        }
        disposeTerminal();
      });

      return {
        activeLocale,
        activeStepKey,
        advanceWizard,
        adminDisplay,
        adminPasswordMatches,
        adminPasswordRules,
        adminUsername,
        canContinue,
        closeIntegrationModal,
        configuredIntegrationCount,
        connectionLabel,
        currentIntegrationProvider,
        currentStep,
        currentWizardOrdinal,
        disableCurrentIntegration,
        errorMessage,
        footerNote,
        form,
        generateManualMasterKey,
        generatePassword,
        handleAdminEmailInput,
        handleAdminLoginInput,
        handleRestoreFileSelection,
        handleRestoreKeyFileSelection,
        handleSecurityEntranceToggle,
        integrationModal,
        integrationLibraryOpen,
        integrationProviders: integrationCatalog,
        isFinalWizardStep,
        isIntegrationConfigured,
        isIntegrationStateValid,
        isStepAdvanced,
        isVisible,
        integrationPickerKey,
        localeLabel,
        localeOptions,
        masterKeyModeLabel,
        mode,
        modeLabel,
        nextWizardStep,
        normalizeDomainInput,
        openIntegrationModal,
        openIntegrationLibrary,
        closeIntegrationLibrary,
        openIntegrationFromLibrary,
        openPickedIntegrationModal,
        panelTargetDisplay,
        panelTargetPreview,
        panelTargetUsesIP,
        previousWizardStep,
        primaryActionLabel,
        restartInstaller,
        restoreInputMode,
        restoreSummaryLabel,
        saveIntegrationModal,
        securityEntranceCookiePath,
        securityEntranceEntryPath,
        securityEntranceSuggested,
        securityPlan,
        securityPlanLabel,
        selectInstallMode,
        selectSecurityPlan,
        selectSSLMode,
        setStepDetailMode,
        setRestoreInputMode,
        showToast,
        sslSummaryLabel,
        startRestoreWizard,
        statusBadgeClass,
        submitSetup,
        submitting,
        suggestedAdminUsername,
        successUrl,
        selectedIntegrationProviders,
        switchLocale,
        t,
        terminalEl,
        themeModeLabel,
        themeModeResolved,
        toggleTheme,
        toast,
        totalWizardSteps,
        suggestSecurityEntranceForIp,
        visibleWizardIndex,
        visibleWizardTimeline,
        toggleVisibility,
        wizardStep
      };
    }
  }).use(i18n).mount('#app');
}

bootstrap();
