import { useState } from "react";
import { Loader2 } from "lucide-react";
import { SaveProviderConfig, SaveProviderKey, SaveProviderModel, SetProvider } from "../../wailsjs/go/main/App";
import { useI18n, LANGUAGES, Lang } from "../i18n";
import { Button } from "./ui/button";
import { Dialog, DialogContent, DialogTitle } from "./ui/dialog";
import { Input } from "./ui/input";
import { Label } from "./ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "./ui/select";
import { Tabs, TabsList, TabsTrigger } from "./ui/tabs";

export interface IProviderInfo {
  hasKey: boolean;
  keyPreview: string;
  model: string;
  models?: string[]; // 선택 가능한 모델 목록 (최신 모델이 맨 앞)
  baseUrl?: string; // custom(OpenAI 호환) Base URL (원문 노출)
  accountId?: string; // Cloudflare Workers AI 계정 ID (원문 노출)
}

export interface ISettings {
  provider: string;
  providers: Record<string, IProviderInfo>;
}

interface IProps {
  settings: ISettings;
  onClose: () => void;
  onSaved: (msg: string, keepOpen?: boolean) => void;
}

// 자격 증명 입력 필드 정의 (단일 키 프로바이더는 key 하나, cloudflare는 accountId+key,
// custom은 baseUrl+key)
type CredFieldId = "key" | "accountId" | "baseUrl";

interface ICredField {
  id: CredFieldId;
  labelKey: string;
  placeholder: string;
  type?: "password" | "text";
  required?: boolean;
}

interface IProviderMeta {
  key: string;
  label: string;
  fields: ICredField[];
}

const PROVIDERS: IProviderMeta[] = [
  {
    key: "gemini",
    label: "Gemini",
    fields: [{ id: "key", labelKey: "api_key", placeholder: "AIza...", type: "password", required: true }],
  },
  {
    key: "openai",
    label: "OpenAI",
    fields: [{ id: "key", labelKey: "api_key", placeholder: "sk-...", type: "password", required: true }],
  },
  {
    key: "openrouter",
    label: "OpenRouter",
    fields: [{ id: "key", labelKey: "api_key", placeholder: "sk-or-...", type: "password", required: true }],
  },
  {
    key: "fal",
    label: "fal.ai",
    fields: [{ id: "key", labelKey: "api_key", placeholder: "key_id:key_secret", type: "password", required: true }],
  },
  {
    key: "byteplus",
    label: "BytePlus",
    fields: [{ id: "key", labelKey: "api_key", placeholder: "ark-...", type: "password", required: true }],
  },
  {
    key: "cloudflare",
    label: "Cloudflare",
    fields: [
      { id: "accountId", labelKey: "account_id", placeholder: "your-account-id", type: "text", required: true },
      { id: "key", labelKey: "api_token", placeholder: "your-api-token", type: "password", required: true },
    ],
  },
  {
    key: "custom",
    label: "Custom",
    fields: [
      { id: "baseUrl", labelKey: "base_url", placeholder: "https://api.example.com", type: "text", required: true },
      { id: "key", labelKey: "api_key_optional", placeholder: "sk-... (optional)", type: "password", required: false },
    ],
  },
];

// 멀티 프로바이더 설정 모달 (shadcn Dialog)
export default function SettingsModal({ settings, onClose, onSaved }: IProps) {
  const { t, lang, setLang } = useI18n();
  const activeHasKey = !!settings.providers?.[settings.provider]?.hasKey;
  const [tab, setTab] = useState(settings.provider || "gemini");
  const [values, setValues] = useState<Record<string, string>>({});
  const [model, setModel] = useState(settings.providers?.[settings.provider || "gemini"]?.model ?? "");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  const info = settings.providers?.[tab];
  const meta = PROVIDERS.find((p) => p.key === tab)!;
  const isActive = settings.provider === tab;
  const isMultiField = meta.key === "cloudflare" || meta.key === "custom";

  const switchTab = (k: string) => {
    setTab(k);
    setValues({});
    setModel(settings.providers?.[k]?.model ?? "");
    setError("");
  };

  // 저장된 값 미리보기: 키는 마스킹, AccountID/BaseURL은 원문 표시
  const savedPreview = (field: ICredField): string => {
    switch (field.id) {
      case "accountId":
        return info?.accountId ?? "";
      case "baseUrl":
        return info?.baseUrl ?? "";
      default:
        return info?.hasKey && info?.keyPreview ? t("saved_as", { preview: info.keyPreview }) : "";
    }
  };

  const canSave =
    !busy && meta.fields.every((f) => !f.required || (values[f.id] ?? "").trim() !== "");

  const saveCreds = async () => {
    if (busy || !canSave) return;
    setBusy(true);
    setError("");
    try {
      if (isMultiField) {
        await SaveProviderConfig(meta.key, {
          apiKey: (values["key"] ?? "").trim(),
          baseUrl: (values["baseUrl"] ?? "").trim(),
          accountId: (values["accountId"] ?? "").trim(),
        });
        setValues({});
        onSaved(t("toast_config_saved", { provider: meta.label }));
      } else {
        const key = (values["key"] ?? "").trim();
        await SaveProviderKey(meta.key, key);
        setValues({});
        onSaved(t("toast_key_saved", { provider: meta.label }));
      }
    } catch (e) {
      setError(String(e));
    } finally {
      setBusy(false);
    }
  };

  const saveModel = async (explicit?: string) => {
    if (busy) return;
    const next = (explicit ?? model).trim();
    if (explicit !== undefined) setModel(next);
    setBusy(true);
    setError("");
    try {
      await SaveProviderModel(tab, next);
      onSaved(t("toast_model_saved", { provider: meta.label }), true);
    } catch (e) {
      setError(String(e));
    } finally {
      setBusy(false);
    }
  };

  const activate = async () => {
    if (busy) return;
    setBusy(true);
    setError("");
    try {
      await SetProvider(tab);
      onSaved(t("toast_provider_active", { provider: meta.label }));
    } catch (e) {
      setError(String(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog open onOpenChange={(open) => !open && activeHasKey && onClose()}>
      <DialogContent hideClose={!activeHasKey}>
        <DialogTitle>{t("settings")}</DialogTitle>

        <div className="field">
          <Label>{t("language")}</Label>
          <Select value={lang} onValueChange={(v) => setLang(v as Lang)}>
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {LANGUAGES.map((l) => (
                <SelectItem key={l.code} value={l.code}>
                  {l.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <Tabs value={tab} onValueChange={switchTab}>
          <TabsList>
            {PROVIDERS.map((p) => (
              <TabsTrigger key={p.key} value={p.key}>
                {p.label}
                {settings.providers?.[p.key]?.hasKey && <span className="ml-1 text-success">●</span>}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>

        {meta.fields.map((field) => (
          <div className="field" key={field.id}>
            <Label>
              {t(field.labelKey, { provider: meta.label })} {isActive && field.id === "key" && <em className="not-italic text-primary">{t("in_use")}</em>}
            </Label>
            <Input
              type={field.type === "text" ? "text" : "password"}
              placeholder={savedPreview(field) || field.placeholder}
              value={values[field.id] ?? ""}
              onChange={(e) => setValues((v) => ({ ...v, [field.id]: e.target.value }))}
              onKeyDown={(e) => e.key === "Enter" && saveCreds()}
              autoFocus={meta.fields[0]?.id === field.id}
            />
          </div>
        ))}

        <div className="field">
          <Label>{t("image_model")}</Label>
          {(info?.models?.length ?? 0) > 0 && (
            <Select
              value={info!.models!.includes(model.trim()) ? model.trim() : undefined}
              onValueChange={(v) => v !== model.trim() && saveModel(v)}
            >
              <SelectTrigger disabled={busy}>
                <SelectValue placeholder={t("custom_model_inuse")} />
              </SelectTrigger>
              <SelectContent>
                {info!.models!.map((m, i) => (
                  <SelectItem key={m} value={m}>
                    {m}
                    {i === 0 && t("latest")}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
          <div className="row">
            <Input
              className="flex-1"
              placeholder={info?.model ?? ""}
              value={model}
              onChange={(e) => setModel(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && model.trim() !== (info?.model ?? "") && saveModel()}
            />
            <Button
              variant="outline"
              size="sm"
              onClick={() => saveModel()}
              disabled={busy || model.trim() === (info?.model ?? "")}
              title={t("model_apply_tip")}
            >
              {t("apply")}
            </Button>
          </div>
        </div>

        <p className="hint">
          {t(`help_${tab}`)}
          <br />{t("key_local_note")}
        </p>

        {error && <p className="hint" style={{ color: "hsl(var(--destructive))" }}>{error}</p>}

        <div className="row justify-end">
          {activeHasKey && (
            <Button variant="ghost" onClick={onClose}>
              {t("close")}
            </Button>
          )}
          {!isActive && info?.hasKey && (
            <Button variant="outline" onClick={activate} disabled={busy}>
              {t("use_provider")}
            </Button>
          )}
          <Button onClick={saveCreds} disabled={!canSave}>
            {busy && <Loader2 size={13} className="animate-spin" />}
            {busy ? t("checking") : isMultiField ? t("save_config") : t("save_key")}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}