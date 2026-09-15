"use client";

import { useState, type FormEvent } from "react";

import { Button } from "@/components/ui/button";
import {
  AuthApiError,
  login,
  register,
  type AuthSession,
  writeAuthSession,
} from "@/lib/api/auth";

type AuthValues = {
  email: string;
  password: string;
  firstName: string;
  lastName: string;
};

type AuthErrors = Partial<Record<keyof AuthValues, string>> & { form?: string };

type AuthPanelProps = {
  onAuthenticated: (session: AuthSession) => void;
  title?: string;
  description?: string;
};

const initialValues: AuthValues = { email: "", password: "", firstName: "", lastName: "" };
const inputClass = "mt-2 h-11 w-full rounded-lg border border-border bg-background px-3 text-sm text-foreground outline-none placeholder:text-muted/70 focus:border-ring focus:ring-2 focus:ring-ring/30";
const labelClass = "text-xs font-medium text-foreground";

function FieldError({ message }: { message?: string }) {
  return message ? <p className="mt-1 text-xs text-red-200">{message}</p> : null;
}

export function AuthPanel({
  onAuthenticated,
  title = "Etkinlik oluşturmak için giriş yap",
  description = "Etkinlik API’si yalnızca doğrulanmış kullanıcılar için açık.",
}: AuthPanelProps) {
  const [mode, setMode] = useState<"login" | "register">("login");
  const [values, setValues] = useState<AuthValues>(initialValues);
  const [errors, setErrors] = useState<AuthErrors>({});
  const [pending, setPending] = useState(false);

  function updateValue(key: keyof AuthValues, value: string) {
    setValues((current) => ({ ...current, [key]: value }));
    setErrors((current) => ({ ...current, [key]: undefined, form: undefined }));
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const nextErrors: AuthErrors = {};
    if (!values.email.trim()) nextErrors.email = "E-posta gerekli.";
    if (!values.password) nextErrors.password = "Şifre gerekli.";
    if (values.password && values.password.length < 8) nextErrors.password = "Şifre en az 8 karakter olmalı.";
    if (mode === "register" && !values.firstName.trim()) nextErrors.firstName = "Ad gerekli.";
    if (mode === "register" && !values.lastName.trim()) nextErrors.lastName = "Soyad gerekli.";
    if (Object.keys(nextErrors).length > 0) {
      setErrors(nextErrors);
      return;
    }

    setPending(true);
    setErrors({});
    try {
      if (mode === "register") {
        await register({ email: values.email.trim(), password: values.password, first_name: values.firstName.trim(), last_name: values.lastName.trim() });
      }
      const session = await login({ email: values.email.trim(), password: values.password });
      writeAuthSession(session);
      onAuthenticated(session);
    } catch (error) {
      const message = error instanceof AuthApiError ? error.message : "Oturum açılamadı. Tekrar deneyebilirsin.";
      setErrors({ form: message });
    } finally {
      setPending(false);
    }
  }

  return (
    <section className="rounded-xl border border-border bg-surface p-5 sm:p-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div><p className="text-xs font-medium uppercase tracking-[0.16em] text-accent">Kimlik doğrulama</p><h2 className="mt-1 text-xl font-semibold text-foreground">{title}</h2><p className="mt-2 text-sm leading-6 text-muted">{description}</p></div>
        <div className="flex rounded-lg border border-border bg-background p-1" role="tablist" aria-label="Kimlik doğrulama seçimi">
          <button type="button" role="tab" aria-selected={mode === "login"} onClick={() => { setMode("login"); setErrors({}); }} className={`rounded-md px-3 py-2 text-xs font-medium ${mode === "login" ? "bg-surface-hover text-foreground" : "text-muted"}`}>Giriş yap</button>
          <button type="button" role="tab" aria-selected={mode === "register"} onClick={() => { setMode("register"); setErrors({}); }} className={`rounded-md px-3 py-2 text-xs font-medium ${mode === "register" ? "bg-surface-hover text-foreground" : "text-muted"}`}>Kayıt ol</button>
        </div>
      </div>

      <form className="mt-6 space-y-4" onSubmit={handleSubmit}>
        {mode === "register" ? <div className="grid gap-4 sm:grid-cols-2"><label className={labelClass}>Ad<input className={inputClass} value={values.firstName} onChange={(event) => updateValue("firstName", event.target.value)} autoComplete="given-name" /> <FieldError message={errors.firstName} /></label><label className={labelClass}>Soyad<input className={inputClass} value={values.lastName} onChange={(event) => updateValue("lastName", event.target.value)} autoComplete="family-name" /> <FieldError message={errors.lastName} /></label></div> : null}
        <div className="grid gap-4 sm:grid-cols-2"><label className={labelClass}>E-posta<input type="email" className={inputClass} value={values.email} onChange={(event) => updateValue("email", event.target.value)} autoComplete="email" /> <FieldError message={errors.email} /></label><label className={labelClass}>Şifre<input type="password" className={inputClass} value={values.password} onChange={(event) => updateValue("password", event.target.value)} autoComplete={mode === "login" ? "current-password" : "new-password"} /> <FieldError message={errors.password} /></label></div>
        {errors.form ? <p className="rounded-lg border border-red-300/20 bg-red-400/10 px-3 py-2 text-sm text-red-100">{errors.form}</p> : null}
        <Button type="submit" disabled={pending}>{pending ? "Bekleniyor…" : mode === "login" ? "Giriş yap" : "Kayıt ol ve devam et"}</Button>
      </form>
    </section>
  );
}
