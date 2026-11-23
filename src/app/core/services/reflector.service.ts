import { Injectable, inject, signal, DestroyRef } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { DOCUMENT } from '@angular/common';
import { BehaviorSubject, Subject, Observable, interval, merge, switchMap, startWith, catchError, of, tap, map, shareReplay } from 'rxjs';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { ReflectorDashboardData, YsfStation, YsfActivity, DmrActivity, DVSwitchActivity, DVSwitchBridge, EchoLinkStation, EchoLinkLastHeard } from '../models/reflector.model';

/** Forma devuelta por DashboardStatusResponse. */
interface BackendStatusRaw {
  header: {
    title: string;
    subtitle: string;
    badge: string;
    localBridgeStatus: string;
    lastUpdate: string;
  };
  stats: {
    ysfConnectedNow: number;
    ysfTx24h: number;
    dmrTx24h: number;
    dvSwitchTx24h?: number;
    stations24h: number;
  };
  dvSwitchTitle?: string | null;
  dvSwitchBridges?: DVSwitchBridge[] | null;
  ysfLastTx?: Array<{ callsign: string; duration: string; ago: string; target?: string; gateway?: string }> | null;
  dmrLastTx?: Array<{ callsign: string; duration: string; ago: string; dmrId?: string; talkgroup?: string }> | null;
  dvSwitchLastTx?: Array<{
    rxTime: string; mode: string; callsign: string; name: string; target: string;
    source: string; duration: string; loss: string; ber: string;
  }> | null;
  ysfConnectedList?: Array<{ callsign: string; ip: string; type: string; since: string }> | null;
  combined24h?: Array<{
    time: string;
    mode: string;
    callsign: string;
    duration: string;
    detail: string;
  }> | null;
  echolinkConnectedList?: EchoLinkStation[] | null;
  echolinkLastHeard?: EchoLinkLastHeard[] | null;
  echolinkUptime?: string | null;
  ysfAvailable?: boolean;
  ysfTrafficAvailable?: boolean;
  dmrAvailable?: boolean;
  echolinkAvailable?: boolean;
  warnings?: string[] | null;
}

export interface SiteProfile {
  id: string;
  name: string;
  visible: boolean;
  url?: string;
  header?: {
    title: string;
    subtitle: string;
    badge: string;
    dvSwitchTitle?: string;
  } | null;
}

@Injectable({
  providedIn: 'root'
})
export class ReflectorService {
  private readonly http = inject(HttpClient);
  private readonly document = inject(DOCUMENT);
  private readonly destroyRef = inject(DestroyRef);

  private readonly statusUrl = this.resolveStatusUrl();
  private readonly selectedProfile = new BehaviorSubject<string>('');
  private readonly refreshRequests = new Subject<void>();

  readonly data = signal<ReflectorDashboardData | null>(null);
  readonly error = signal<string | null>(null);
  readonly loading = signal<boolean>(true);

  /**
   * Resuelve la ruta del endpoint respetando <base href="/ameriboard/">
   * cuando el binario Go embebe el SPA bajo BASE_PATH.
   */
  private resolveStatusUrl(): string {
    const baseHref = this.document.querySelector('base')?.getAttribute('href') ?? '/';
    const normalizedBase = baseHref.endsWith('/') ? baseHref.slice(0, -1) : baseHref;
    return `${normalizedBase}/api/reflector-status`;
  }

  /** Petición única normalizada */
  fetchStatusOnce(profileId = ''): Observable<ReflectorDashboardData | null> {
    const url = profileId ? this.statusUrl.replace('/reflector-status', `/profiles/${encodeURIComponent(profileId)}/status`) : this.statusUrl;
    return this.http.get<BackendStatusRaw>(url).pipe(
      map((raw) => {
        if (!raw?.header || !raw.stats) {
          throw new Error('Respuesta de telemetría incompleta');
        }
        return this.mapBackendToModel(raw);
      }),
      tap((mapped) => {
        this.data.set(mapped);
        this.error.set(null);
        this.loading.set(false);
      }),
      catchError(() => {
        this.data.set(null);
        this.error.set('Sin conexión con el servidor de telemetría');
        this.loading.set(false);
        return of(null);
      })
    );
  }

  /**
   * Polling reactivo cada N ms (por defecto 5000 ms) que también
   * actualiza los signals internos del servicio.
   */
  pollStatus(periodMs = 5000): Observable<ReflectorDashboardData | null> {
    return this.selectedProfile.pipe(
      switchMap(profileId => merge(interval(periodMs), this.refreshRequests).pipe(
        startWith(0),
        switchMap(() => this.fetchStatusOnce(profileId))
      )),
      shareReplay({ bufferSize: 1, refCount: true }),
      takeUntilDestroyed(this.destroyRef)
    );
  }

  refresh(): void {
    this.refreshRequests.next();
  }

  selectProfile(id: string): void {
    this.data.set(null);
    this.error.set(null);
    this.loading.set(true);
    this.selectedProfile.next(id);
  }

  getProfiles(): Observable<SiteProfile[]> {
    return this.http.get<SiteProfile[]>(this.statusUrl.replace('/reflector-status', '/profiles'));
  }

  private mapBackendToModel(raw: BackendStatusRaw): ReflectorDashboardData {
    const ysfConnected: YsfStation[] = (raw.ysfConnectedList ?? []).map((item) => ({
      callsign: item.callsign,
      ip: item.ip,
      gateway: item.type,
      since: item.since
    }));

    const ysfLastHeard: YsfActivity[] = this.uniqueLastHeard(raw.ysfLastTx ?? []).map((tx) => ({
      time: tx.ago,
      callsign: tx.callsign,
      target: tx.target || '—',
      gateway: tx.gateway || '—',
      duration: tx.duration,
      loss: '—'
    }));

    const dmrLastHeard: DmrActivity[] = this.uniqueLastHeard(raw.dmrLastTx ?? []).map((tx) => ({
      time: tx.ago,
      callsign: tx.callsign,
      dmrId: tx.dmrId || '—',
      talkgroup: tx.talkgroup ? `TG ${tx.talkgroup}` : '—',
      source: '—',
      duration: tx.duration
    }));

    const dvSwitchLastHeard: DVSwitchActivity[] = (raw.dvSwitchLastTx ?? []).map((tx) => ({
      rxTime: tx.rxTime.replace(/^(\d{2})-(\d{2}) (\d{2}:\d{2}:\d{2})$/, '$2/$1 $3'),
      mode: tx.mode,
      callsign: tx.callsign,
      name: tx.name || '—',
      target: tx.target || '—',
      source: tx.source || '—',
      duration: tx.duration || '—',
      loss: tx.loss || '—',
      ber: tx.ber || '—'
    }));

    const echolinkConnected: EchoLinkStation[] = (raw.echolinkConnectedList ?? []).map((station) => ({
      number: station.number,
      user: station.user,
      attribute: station.attribute
    }));
    const echolinkLastHeard: EchoLinkLastHeard[] = (raw.echolinkLastHeard ?? []).map((entry) => ({
      callsign: entry.callsign,
      disconnectedAt: entry.disconnectedAt,
      duration: entry.duration
    }));

    return {
      ysfAvailable: raw.ysfAvailable !== false,
      ysfTrafficAvailable: raw.ysfTrafficAvailable ?? raw.ysfAvailable !== false,
      dmrAvailable: raw.dmrAvailable !== false,
      echolinkAvailable: raw.echolinkAvailable !== false,
      warnings: (raw.warnings ?? []).filter(warning =>
        !((raw.ysfAvailable === false && warning === 'No se encontraron logs YSF recientes') ||
          (raw.dmrAvailable === false && warning === 'No se encontraron logs DMR recientes') ||
          (raw.echolinkAvailable === false && warning === 'EchoLink no está disponible'))),
      header: raw.header,
      stats: {
        ysfConnectedCount: raw.stats.ysfConnectedNow,
        ysfTx24h: raw.stats.ysfTx24h,
        dmrTx24h: raw.stats.dmrTx24h,
        dvSwitchTx24h: raw.stats.dvSwitchTx24h ?? raw.stats.dmrTx24h,
        stations24h: raw.stats.stations24h
      },
      dvSwitchTitle: raw.dvSwitchTitle?.trim() || 'Actividad digital vía DVSwitch',
      dvSwitchBridges: raw.dvSwitchBridges ?? [],
      ysfConnected,
      ysfLastHeard,
      dmrLastHeard,
      dvSwitchLastHeard,
      echolinkConnected,
      echolinkLastHeard,
      echolinkUptime: raw.echolinkUptime?.trim() || '—'
    };
  }

  private uniqueLastHeard<T extends { callsign: string }>(entries: T[]): T[] {
    const seen = new Set<string>();
    return entries.filter((entry) => {
      const callsign = entry.callsign.trim().toUpperCase();
      if (!callsign || seen.has(callsign)) {
        return false;
      }
      seen.add(callsign);
      return true;
    });
  }
}
