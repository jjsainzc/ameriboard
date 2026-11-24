import { DatePipe } from '@angular/common';
import { HttpClient, HttpErrorResponse, HttpHeaders } from '@angular/common/http';
import { DOCUMENT } from '@angular/common';
import { Component, computed, DestroyRef, ElementRef, inject, OnInit, signal, viewChildren } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { RouterLink } from '@angular/router';
import { ReflectorService, SiteProfile } from '../../core/services/reflector.service';
import { EchoLinkStation } from '../../core/models/reflector.model';
import { TablePagination, TablePaginationComponent } from '../../shared/table-pagination/table-pagination.component';

@Component({
  selector: 'app-reflector-dashboard',
  imports: [RouterLink, DatePipe, ReactiveFormsModule, TablePaginationComponent],
  templateUrl: './reflector-dashboard.component.html',
  styleUrl: './reflector-dashboard.components.scss',
  host: { '(document:click)': 'closeMenusOnOutsideClick($event)' }
})
export class ReflectorDashboardComponent implements OnInit {
  private readonly reflectorService = inject(ReflectorService);
  private readonly destroyRef = inject(DestroyRef);
  private readonly http = inject(HttpClient);
  private readonly document = inject(DOCUMENT);
  private readonly fb = inject(FormBuilder).nonNullable;
  private readonly base = (this.document.querySelector('base')?.getAttribute('href') ?? '/').replace(/\/$/, '');
  private readonly headers = new HttpHeaders({ 'X-Ameriboard-Request': '1' });
  private readonly dropdownMenus = viewChildren<ElementRef<HTMLDetailsElement>>('dropdownMenu');

  readonly data = this.reflectorService.data;
  readonly error = this.reflectorService.error;
  readonly loading = this.reflectorService.loading;
  readonly profiles = signal<SiteProfile[]>([]);
  readonly profilesError = signal('');
  readonly echolinkAdmin = signal(false);
  readonly adminSetupRequired = signal<boolean | null>(null);
  readonly adminBusy = signal(false);
  readonly adminError = signal('');
  readonly adminPasswordVisible = signal(false);
  readonly adminForm = this.fb.group({
    password: ['', [Validators.required, Validators.maxLength(72)]],
    confirm: ['']
  });
  readonly echolinkActionBusy = signal(false);
  readonly pendingEchoLinkAction = signal<{ action: 'connect' | 'disconnect'; callsign: string } | null>(null);
  readonly echolinkActionNotice = signal('');
  readonly echolinkActionError = signal('');
  readonly selectedProfileId = signal('');
  readonly actionDialogStation = signal('');
  readonly actionDialogAction = signal('');
  readonly actionDialogInput = signal('');
  readonly actionDialogIp = signal('');
  readonly actionDialogPermanent = signal(false);
  readonly bannedStations = signal<string[]>([]);
  readonly pendingRetries = signal<{ target: string; attempts: number }[]>([]);
  readonly ysfConnectedPages = new TablePagination(computed(() => this.data()?.ysfConnected ?? []));
  readonly ysfLastHeardPages = new TablePagination(computed(() => this.data()?.ysfLastHeard ?? []));
  readonly echolinkConnectedPages = new TablePagination(computed(() => this.data()?.echolinkConnected ?? []));
  readonly pendingEchoLinkStationVisible = computed(() =>
    this.echolinkConnectedPages.rows().some(station => this.pendingStationAction(station.user) !== null)
  );
  readonly echolinkLastHeardPages = new TablePagination(computed(() => this.data()?.echolinkLastHeard ?? []), station => [
    station.callsign, station.duration, station.disconnectedAt,
    new DatePipe('en-US').transform(station.disconnectedAt, 'dd/MM/yyyy HH:mm:ss') ?? ''
  ]);
  readonly dvSwitchPages = new TablePagination(computed(() => this.data()?.dvSwitchLastHeard ?? []), undefined, true);
  readonly echolinkAttributes = [
    { code: 'A', meaning: 'Sesión iniciada como administrador.' },
    { code: 'a', meaning: 'Códec ADPCM.' },
    { code: 'B', meaning: 'Conferencia theBridge enlazada.' },
    { code: 'C', meaning: 'Conferencia enlazada distinta de theBridge.' },
    { code: 'c', meaning: 'Texto del chat suprimido.' },
    { code: 'F', meaning: 'Reproduciendo un archivo mediante .play o .test.' },
    { code: 'f', meaning: 'Conexión full dúplex.' },
    { code: 'K', meaning: 'Expulsado.' },
    { code: 'L', meaning: 'Oyente pasivo.' },
    { code: 'M', meaning: 'Audio y texto silenciados.' },
    { code: 'm', meaning: 'Audio silenciado.' },
    { code: 'P', meaning: 'Conexión permanente mediante .connect.' },
    { code: 'R', meaning: 'Solo recepción (monitoreado).' },
    { code: 'S', meaning: 'Sesión iniciada como operador del sistema (sysop).' },
    { code: 'T', meaning: 'Transmitiendo en este momento.' },
    { code: 'u', meaning: 'Códec uLaw.' },
    { code: 'x', meaning: 'Actualmente inactivo en la conferencia.' },
    { code: '*', meaning: 'Usa el protocolo Asterisk.' },
    { code: '0', meaning: 'Usa el protocolo Speak Freely.' },
    { code: '1', meaning: 'Usa el protocolo RTP.' },
    { code: '!', meaning: 'Versión antigua de theBridge que envía paquetes SDES con una extensión privada "txt".' }
  ];

  ngOnInit(): void {
    this.reflectorService.selectProfile('');
    this.reflectorService.pollStatus(5000).pipe(takeUntilDestroyed(this.destroyRef)).subscribe();
    this.reflectorService.getProfiles().pipe(takeUntilDestroyed(this.destroyRef)).subscribe({
      next: profiles => this.profiles.set(profiles),
      error: () => this.profilesError.set('No se pudo cargar la lista de sitios; el dashboard local sigue disponible.')
    });
    this.http.get(`${this.base}/api/admin/profiles`).pipe(takeUntilDestroyed(this.destroyRef)).subscribe({
      next: () => this.echolinkAdmin.set(true),
      error: () => this.echolinkAdmin.set(false)
    });
  }

  selectProfile(event: Event): void {
    for (const pagination of [this.ysfConnectedPages, this.ysfLastHeardPages, this.echolinkConnectedPages,
      this.echolinkLastHeardPages, this.dvSwitchPages]) {
      pagination.reset();
    }
    const profileId = (event.target as HTMLSelectElement).value;
    this.selectedProfileId.set(profileId);
    this.echolinkActionError.set('');
    this.echolinkActionNotice.set('');
    this.bannedStations.set([]);
    this.reflectorService.selectProfile(profileId);
  }

  refresh(): void {
    this.reflectorService.refresh();
  }

  closeMenusOnOutsideClick(event: MouseEvent): void {
    if (event.target instanceof Node) {
      for (const menu of this.dropdownMenus()) {
        if (!menu.nativeElement.contains(event.target)) { menu.nativeElement.open = false; }
      }
    }
  }

  openAdminLogin(dialog: HTMLDialogElement): void {
    this.adminForm.reset();
    this.adminPasswordVisible.set(false);
    this.adminError.set('');
    this.adminSetupRequired.set(null);
    dialog.showModal();
    this.http.get<{ required: boolean }>(`${this.base}/api/admin/setup`)
      .pipe(takeUntilDestroyed(this.destroyRef)).subscribe({
        next: result => this.adminSetupRequired.set(result.required),
        error: () => this.adminError.set('No se pudo consultar la configuración del administrador.')
      });
  }

  closeAdminLogin(dialog: HTMLDialogElement): void {
    this.adminForm.reset();
    this.adminPasswordVisible.set(false);
    this.adminError.set('');
    dialog.close();
  }

  toggleAdminPasswordVisibility(): void {
    this.adminPasswordVisible.update(visible => !visible);
    this.adminError.set('');
  }

  cancelAdminDialog(event: Event): void {
    if (this.adminBusy()) {
      event.preventDefault();
      return;
    }
    this.adminForm.reset();
    this.adminPasswordVisible.set(false);
    this.adminError.set('');
  }

  submitAdminLogin(dialog: HTMLDialogElement): void {
    const setup = this.adminSetupRequired();
    const { password, confirm } = this.adminForm.getRawValue();
    if (this.adminBusy() || setup === null || this.adminForm.invalid) { return; }
    if (setup && (password.length < 8 || (!this.adminPasswordVisible() && password !== confirm))) {
      this.adminError.set('La contraseña debe tener al menos 8 caracteres y coincidir con la confirmación.');
      return;
    }
    this.adminBusy.set(true);
    this.adminError.set('');
    this.http.post(`${this.base}/api/admin/${setup ? 'setup' : 'login'}`, { password }, { headers: this.headers })
      .pipe(takeUntilDestroyed(this.destroyRef)).subscribe({
        next: () => {
          this.adminBusy.set(false);
          this.echolinkAdmin.set(true);
          this.closeAdminLogin(dialog);
          this.loadProfiles();
        },
        error: error => {
          this.adminBusy.set(false);
          this.adminError.set(this.actionErrorMessage(error, 'No se pudo iniciar sesión. Revisa la contraseña.'));
        }
      });
  }

  logoutAdmin(): void {
    if (this.adminBusy()) { return; }
    this.adminBusy.set(true);
    this.adminError.set('');
    this.http.post(`${this.base}/api/admin/logout`, {}, { headers: this.headers })
      .pipe(takeUntilDestroyed(this.destroyRef)).subscribe({
        next: () => {
          this.adminBusy.set(false);
          this.echolinkAdmin.set(false);
          this.selectedProfileId.set('');
          this.reflectorService.selectProfile('');
          this.loadProfiles();
        },
        error: error => {
          this.adminBusy.set(false);
          if (this.handleExpiredAdmin(error)) { return; }
          this.adminError.set(this.actionErrorMessage(error, 'No se pudo cerrar la sesión.'));
        }
      });
  }

  private loadProfiles(): void {
    this.reflectorService.getProfiles().pipe(takeUntilDestroyed(this.destroyRef)).subscribe({
      next: profiles => { this.profiles.set(profiles); this.profilesError.set(''); },
      error: () => this.profilesError.set('No se pudo cargar la lista de sitios; el dashboard local sigue disponible.')
    });
  }

  echolinkMute(station: EchoLinkStation, muted: boolean): void {
    this.executeEchoLinkAction(muted ? 'unmute' : 'mute', station.user);
  }

  echolinkDisconnect(station: EchoLinkStation): void {
    this.executeEchoLinkAction('disconnect', station.user);
  }

  openEchoLinkAction(dialog: HTMLDialogElement, action: string, station: string = ''): void {
    for (const menu of this.dropdownMenus()) { menu.nativeElement.open = false; }
    this.actionDialogAction.set(action);
    this.actionDialogStation.set(station);
    this.actionDialogInput.set(action === 'ban' ? station : '');
    this.actionDialogIp.set('');
    this.actionDialogPermanent.set(false);
    this.echolinkActionError.set('');
    this.echolinkActionNotice.set('');
    if (!dialog.open) { dialog.showModal(); }
    if (action === 'ban-list') { this.loadBannedStations(); }
    if (action === 'retry-list') { this.loadPendingRetries(); }
  }

  submitEchoLinkAction(dialog: HTMLDialogElement): void {
    const action = this.actionDialogAction();
    const station = this.actionDialogStation();
    const body: Record<string, string | boolean> = { action };
    if (action === 'connect') {
      body['callsign'] = this.actionDialogInput();
      body['permanent'] = this.actionDialogPermanent();
    } else if (action === 'ban') {
      body['callsign'] = this.actionDialogInput();
      body['ip'] = this.actionDialogIp();
    } else if (action === 'disconnect') {
      body['callsign'] = station;
    } else if (action === 'message') {
      body['message'] = this.actionDialogInput();
    }
    this.executeEchoLinkAction(action, String(body['callsign'] ?? ''), body, dialog);
  }

  private executeEchoLinkAction(action: string, callsign: string = '', body?: Record<string, string | boolean>, dialog?: HTMLDialogElement): void {
    if (this.echolinkActionBusy()) { return; }
    this.echolinkActionBusy.set(true);
    if (action === 'connect' || action === 'disconnect') {
      this.pendingEchoLinkAction.set({ action, callsign: callsign.trim() });
      if (action === 'connect') { dialog?.close(); }
    }
    this.echolinkActionNotice.set('');
    this.echolinkActionError.set('');
    const profileId = this.selectedProfileId();
    const url = this.echoLinkActionUrl(profileId);
    const payload = body ?? { action, callsign };
    this.http.post<{ output?: string }>(url, payload, { headers: this.headers })
      .pipe(takeUntilDestroyed(this.destroyRef)).subscribe({
        next: response => {
          this.echolinkActionBusy.set(false);
          this.pendingEchoLinkAction.set(null);
          if (action === 'connect' || action === 'disconnect') {
            this.echolinkActionNotice.set(action === 'connect' && response.output?.startsWith('No se pudo conectar')
              ? response.output : '');
          } else {
            this.echolinkActionNotice.set(response.output || 'Acción EchoLink ejecutada.');
          }
          if (action === 'unban') {
            this.loadBannedStations();
          } else if (action === 'retry-cancel') {
            this.loadPendingRetries();
          } else {
            if (action !== 'connect') { dialog?.close(); }
          }
          this.reflectorService.refresh();
        },
        error: error => {
          this.echolinkActionBusy.set(false);
          this.pendingEchoLinkAction.set(null);
          if (this.handleExpiredAdmin(error)) { return; }
          this.echolinkActionError.set(this.actionErrorMessage(error, 'No se pudo ejecutar la acción EchoLink.'));
        }
      });
  }

  pendingStationAction(callsign: string): 'connect' | 'disconnect' | null {
    const pending = this.pendingEchoLinkAction();
    return pending && pending.callsign.toUpperCase() === callsign.trim().toUpperCase() ? pending.action : null;
  }

  unbanStation(callsign: string): void {
    this.executeEchoLinkAction('unban', callsign);
  }

  private loadBannedStations(): void {
    if (this.echolinkActionBusy()) { return; }
    this.echolinkActionBusy.set(true);
    this.bannedStations.set([]);
    const profileId = this.selectedProfileId();
    this.http.post<{ output: string }>(this.echoLinkActionUrl(profileId), { action: 'ban-list' }, { headers: this.headers })
      .pipe(takeUntilDestroyed(this.destroyRef)).subscribe({
        next: response => {
          this.echolinkActionBusy.set(false);
          if (profileId !== this.selectedProfileId()) { return; }
          try {
            this.bannedStations.set(parseBannedStations(response.output));
            this.echolinkActionError.set('');
          } catch {
            this.echolinkActionError.set('No se pudo interpretar la lista de baneados. Revisa los registros del servidor.');
          }
        },
        error: error => {
          this.echolinkActionBusy.set(false);
          if (this.handleExpiredAdmin(error)) { return; }
          this.echolinkActionError.set(this.actionErrorMessage(error, 'No se pudo consultar la lista de baneados.'));
        }
      });
  }

  cancelRetry(target: string): void {
    this.executeEchoLinkAction('retry-cancel', target);
  }

  private loadPendingRetries(): void {
    if (this.echolinkActionBusy()) { return; }
    this.echolinkActionBusy.set(true);
    this.pendingRetries.set([]);
    const profileId = this.selectedProfileId();
    this.http.post<{ output: string }>(this.echoLinkActionUrl(profileId), { action: 'retry-list' }, { headers: this.headers })
      .pipe(takeUntilDestroyed(this.destroyRef)).subscribe({
        next: response => {
          this.echolinkActionBusy.set(false);
          if (profileId !== this.selectedProfileId()) { return; }
          this.pendingRetries.set(response.output.split('\n').map(line => line.trim().split(/\s+/))
            .filter(fields => fields.length === 2 && fields[0])
            .map(([target, attempts]) => ({ target, attempts: Number(attempts) })));
        },
        error: error => {
          this.echolinkActionBusy.set(false);
          if (this.handleExpiredAdmin(error)) { return; }
          this.echolinkActionError.set(this.actionErrorMessage(error, 'No se pudieron consultar los reintentos pendientes.'));
        }
      });
  }

  private echoLinkActionUrl(profileId: string): string {
    return profileId
      ? `${this.base}/api/profiles/${encodeURIComponent(profileId)}/echolink/action`
      : `${this.base}/api/admin/echolink/action`;
  }

  private handleExpiredAdmin(error: unknown): boolean {
    if (!(error instanceof HttpErrorResponse) || error.status !== 401) { return false; }
    this.echolinkAdmin.set(false);
    this.selectedProfileId.set('');
    this.adminError.set('La sesión de administrador terminó. Inicia sesión nuevamente.');
    this.reflectorService.selectProfile('');
    this.loadProfiles();
    return true;
  }

  private actionErrorMessage(error: unknown, fallback: string): string {
    return error instanceof HttpErrorResponse && typeof error.error?.message === 'string'
      ? error.error.message : fallback;
  }
}

export function parseBannedStations(output: string): string[] {
  const lines = output.split(/\r\n|\r|\n/).map(line => line.trim()).filter(Boolean);
  if (lines.some(line => /^the banned list is empty\.$/i.test(line))) { return []; }
  const start = lines.findIndex(line => /^banned users:$/i.test(line));
  if (start < 0) { throw new Error('Formato de lista desconocido'); }
  const stations = lines.slice(start + 1).map(line => {
    const match = /^([A-Za-z0-9*][A-Za-z0-9*./_-]*)\s+.+$/.exec(line);
    if (!match) { throw new Error('Entrada de lista inválida'); }
    return match[1];
  });
  if (!stations.length) { throw new Error('Lista incompleta'); }
  return stations;
}
