import { DOCUMENT } from '@angular/common';
import { HttpClient, HttpErrorResponse, HttpHeaders } from '@angular/common/http';
import { Component, DestroyRef, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { RouterLink } from '@angular/router';
import { DEFAULT_ECHOLINK_TITLE, DEFAULT_MONITORING, DEFAULT_NXDN_TITLE, MonitoringOptions, normalizeNXDNTitle, SiteProfile } from '../../core/services/reflector.service';

interface MainSettings {
  title: string;
  subtitle: string;
  dvSwitchTitle: string;
  nxdnTitle?: string;
  echolinkTitle?: string;
  monitoring?: MonitoringOptions | null;
}

@Component({
  selector: 'app-site-admin',
  imports: [ReactiveFormsModule, RouterLink],
  templateUrl: './site-admin.component.html',
  styles: `
    :host { display: block; min-height: 100vh; padding: 1.5rem; background: #0b0e17; color: #e2e8f0; }
    main { max-width: 60rem; margin: auto; }
    a { color: #7dd3fc; }
    form, .site { display: grid; gap: .75rem; background: #131724; border: 1px solid #475569; padding: 1rem; margin: 1rem 0; border-radius: .5rem; }
    label { display: grid; gap: .3rem; }
    input, textarea { color: #f8fafc; background: #0f172a; border: 1px solid #94a3b8; padding: .5rem; font: inherit; }
    textarea { resize: vertical; }
    fieldset { display: grid; min-width: 0; gap: .75rem; border: 1px solid #64748b; }
    .check { display: flex; align-items: center; }
    button { padding: .5rem .75rem; background: #164e63; border: 1px solid #94a3b8; color: white; cursor: pointer; }
    button:disabled { opacity: .6; cursor: not-allowed; }
    .actions { display: flex; flex-wrap: wrap; gap: .5rem; }
    .error { color: #fca5a5; }
  `
})
export class SiteAdminComponent {
  private readonly http = inject(HttpClient);
  private readonly fb = inject(FormBuilder).nonNullable;
  private readonly destroyRef = inject(DestroyRef);
  private readonly document = inject(DOCUMENT);
  private readonly base = (this.document.querySelector('base')?.getAttribute('href') ?? '/').replace(/\/$/, '');
  private readonly url = `${this.base}/api/admin`;
  private readonly headers = new HttpHeaders({ 'X-Ameriboard-Request': '1' });

  readonly authenticated = signal(false);
  readonly setupRequired = signal(false);
  readonly busy = signal(false);
  readonly error = signal('');
  readonly notice = signal('');
  readonly sites = signal<SiteProfile[]>([]);
  readonly mainSettingsLoaded = signal(false);
  readonly siteFormOpen = signal(false);
  readonly editing = signal<string | null>(null);
  readonly deleting = signal<string | null>(null);
  readonly setupPasswordVisible = signal(false);
  readonly loginPasswordVisible = signal(false);
  readonly sitePasswordVisible = signal(false);
  readonly loginForm = this.fb.group({ password: ['', Validators.required] });
  readonly mainSettingsForm = this.fb.group({
    title: ['', [Validators.required, Validators.maxLength(200), Validators.pattern(/\S/)]],
    subtitle: ['', Validators.maxLength(500)],
    dvSwitchTitle: ['Actividad digital vía DVSwitch', [Validators.required, Validators.maxLength(200), Validators.pattern(/\S/)]],
    nxdnTitle: [DEFAULT_NXDN_TITLE, [Validators.required, Validators.maxLength(200), Validators.pattern(/\S/)]],
    echolinkTitle: [DEFAULT_ECHOLINK_TITLE, [Validators.required, Validators.maxLength(200), Validators.pattern(/\S/)]],
    monitoring: this.fb.group({ ...DEFAULT_MONITORING })
  });
  readonly setupForm = this.fb.group({
    password: ['', [Validators.required, Validators.minLength(8), Validators.maxLength(72)]],
    confirm: ['', Validators.required]
  });
  readonly siteForm = this.fb.group({
    name: ['', [Validators.required, Validators.maxLength(100)]],
    url: ['', Validators.required],
    password: [''],
    visible: [false],
    title: ['', Validators.maxLength(200)],
    subtitle: ['', Validators.maxLength(500)],
    badge: ['', Validators.maxLength(100)],
    dvSwitchTitle: ['', Validators.maxLength(200)],
    nxdnTitle: ['', Validators.maxLength(200)],
    echolinkTitle: ['', Validators.maxLength(200)],
    monitoring: this.fb.group({ ...DEFAULT_MONITORING })
  });

  constructor() { this.load(); }

  login(): void {
    if (this.busy() || this.loginForm.invalid) { return; }
    this.busy.set(true);
    this.error.set('');
    this.http.post(`${this.url}/login`, this.loginForm.getRawValue(), { headers: this.headers })
      .pipe(takeUntilDestroyed(this.destroyRef)).subscribe({
        next: () => {
          this.loginForm.reset();
          this.busy.set(false);
          this.load();
        },
        error: err => this.fail(err, 'No se pudo iniciar sesión. Revisa la contraseña.')
      });
  }

  load(): void {
    this.mainSettingsLoaded.set(false);
    this.http.get<SiteProfile[]>(`${this.url}/profiles`).pipe(takeUntilDestroyed(this.destroyRef)).subscribe({
      next: sites => {
        this.sites.set(sites);
        this.authenticated.set(true);
        this.error.set('');
        this.loadMainSettings();
      },
      error: err => {
        this.authenticated.set(false);
        if (err instanceof HttpErrorResponse && err.status === 401) {
          this.http.get<{ required: boolean }>(`${this.url}/setup`)
            .pipe(takeUntilDestroyed(this.destroyRef)).subscribe({
              next: result => this.setupRequired.set(result.required),
              error: error => this.fail(error, 'No se pudo consultar la configuración inicial.')
            });
        } else {
          this.fail(err, 'No se pudieron cargar los perfiles.');
        }
      }
    });
  }

  private loadMainSettings(): void {
    this.http.get<MainSettings>(`${this.url}/settings`)
      .pipe(takeUntilDestroyed(this.destroyRef)).subscribe({
        next: settings => {
          this.mainSettingsForm.setValue({ ...settings, nxdnTitle: normalizeNXDNTitle(settings.nxdnTitle), echolinkTitle: settings.echolinkTitle?.trim() || DEFAULT_ECHOLINK_TITLE, monitoring: settings.monitoring ?? DEFAULT_MONITORING });
          this.mainSettingsLoaded.set(true);
        },
        error: err => this.fail(err, 'No se pudo cargar la configuración principal.')
      });
  }

  saveMainSettings(): void {
    if (this.busy() || this.mainSettingsForm.invalid || !this.mainSettingsLoaded()) { return; }
    this.busy.set(true);
    this.error.set('');
    this.notice.set('');
    this.http.put<MainSettings>(
      `${this.url}/settings`, this.mainSettingsForm.getRawValue(), { headers: this.headers }
    ).pipe(takeUntilDestroyed(this.destroyRef)).subscribe({
      next: settings => {
        this.mainSettingsForm.setValue({ ...settings, nxdnTitle: normalizeNXDNTitle(settings.nxdnTitle), echolinkTitle: settings.echolinkTitle?.trim() || DEFAULT_ECHOLINK_TITLE, monitoring: settings.monitoring ?? DEFAULT_MONITORING });
        this.busy.set(false);
        this.notice.set('Configuración principal guardada.');
      },
      error: err => this.fail(err, 'No se pudo guardar la configuración principal.')
    });
  }

  setup(): void {
    if (this.busy() || this.setupForm.invalid || this.setupForm.value.password !== this.setupForm.value.confirm) { return; }
    this.busy.set(true);
    this.error.set('');
    this.notice.set('');
    const { password } = this.setupForm.getRawValue();
    this.http.post(`${this.url}/setup`, { password }, { headers: this.headers })
      .pipe(takeUntilDestroyed(this.destroyRef)).subscribe({
        next: () => {
          this.busy.set(false);
          this.setupForm.reset();
          this.setupRequired.set(false);
          this.notice.set('Administrador creado.');
          this.load();
        },
        error: err => this.fail(err, 'No se pudo crear el administrador.')
      });
  }

  edit(site: SiteProfile): void {
    this.editing.set(site.id);
    this.siteFormOpen.set(true);
    this.deleting.set(null);
    this.siteForm.setValue({
      name: site.name, url: site.url ?? '', password: '', visible: site.visible,
      title: site.header?.title ?? '', subtitle: site.header?.subtitle ?? '', badge: site.header?.badge ?? '',
      dvSwitchTitle: site.header?.dvSwitchTitle ?? '',
      nxdnTitle: site.header?.nxdnTitle ? normalizeNXDNTitle(site.header.nxdnTitle) : '',
      echolinkTitle: site.header?.echolinkTitle ?? '',
      monitoring: site.monitoring ?? DEFAULT_MONITORING
    });
    this.error.set('');
  }

  cancel(): void {
    this.siteFormOpen.set(false);
    this.editing.set(null);
    this.siteForm.reset({ name: '', url: '', password: '', visible: false, title: '', subtitle: '', badge: '', dvSwitchTitle: '', nxdnTitle: '', monitoring: DEFAULT_MONITORING });
    this.sitePasswordVisible.set(false);
    this.error.set('');
  }

  add(): void {
    this.cancel();
    this.deleting.set(null);
    this.siteFormOpen.set(true);
  }

  save(): void {
    if (this.busy() || this.siteForm.invalid || (!this.editing() && !this.siteForm.value.password)) { return; }
    this.busy.set(true);
    this.error.set('');
    const id = this.editing();
    const { name, url, password, visible, title, subtitle, badge, dvSwitchTitle, nxdnTitle, monitoring } = this.siteForm.getRawValue();
    const profile = { name, url, password, visible, monitoring, header: { title, subtitle, badge, dvSwitchTitle, nxdnTitle } };
    const request = id
      ? this.http.put(`${this.url}/profiles/${encodeURIComponent(id)}`, profile, { headers: this.headers })
      : this.http.post(`${this.url}/profiles`, profile, { headers: this.headers });
    request.pipe(takeUntilDestroyed(this.destroyRef)).subscribe({
      next: () => { this.busy.set(false); this.cancel(); this.load(); },
      error: err => this.fail(err, 'No se pudo guardar el perfil.')
    });
  }

  delete(id: string): void {
    if (this.busy()) { return; }
    this.busy.set(true);
    this.http.delete(`${this.url}/profiles/${encodeURIComponent(id)}`, { headers: this.headers })
      .pipe(takeUntilDestroyed(this.destroyRef)).subscribe({
        next: () => { this.busy.set(false); this.deleting.set(null); if (this.editing() === id) { this.cancel(); } this.load(); },
        error: err => this.fail(err, 'No se pudo eliminar el perfil.')
      });
  }

  logout(): void {
    if (this.busy()) { return; }
    this.busy.set(true);
    this.http.post(`${this.url}/logout`, {}, { headers: this.headers }).pipe(takeUntilDestroyed(this.destroyRef)).subscribe({
      next: () => {
        this.busy.set(false);
        this.authenticated.set(false);
        this.sites.set([]);
        this.mainSettingsLoaded.set(false);
        this.mainSettingsForm.reset({ title: '', subtitle: '', dvSwitchTitle: 'Actividad digital vía DVSwitch', nxdnTitle: DEFAULT_NXDN_TITLE, monitoring: DEFAULT_MONITORING });
        this.cancel();
      },
      error: err => this.fail(err, 'No se pudo cerrar la sesión.')
    });
  }

  private fail(error: unknown, fallback: string): void {
    this.busy.set(false);
    const message = error instanceof HttpErrorResponse && typeof error.error?.message === 'string'
      ? error.error.message : '';
    this.error.set(message || fallback);
    if (error instanceof HttpErrorResponse && error.status === 401 && this.authenticated()) {
      this.authenticated.set(false);
      this.mainSettingsLoaded.set(false);
    }
  }
}
