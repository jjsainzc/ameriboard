import { Injectable, inject, signal, DestroyRef } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { DOCUMENT } from '@angular/common';
import { Observable, interval, switchMap, startWith, catchError, of, tap, map, shareReplay } from 'rxjs';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { ReflectorDashboardData, YsfStation, YsfActivity, DmrActivity } from '../models/reflector.model';

/** Forma cruda devuelta por DashboardStatusResponse en reflector_handler.go */
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
    stations24h: number;
  };
  ysfLastTx?: Array<{ callsign: string; duration: string; ago: string }>;
  dmrLastTx?: Array<{ callsign: string; duration: string; ago: string }>;
  ysfConnectedList?: Array<{ callsign: string; type: string; since: string }>;
  combined24h?: Array<{
    time: string;
    mode: string;
    callsign: string;
    duration: string;
    detail: string;
  }>;
}

@Injectable({
  providedIn: 'root'
})
export class ReflectorService {
  private readonly http = inject(HttpClient);
  private readonly document = inject(DOCUMENT);
  private readonly destroyRef = inject(DestroyRef);

  private readonly statusUrl = this.resolveStatusUrl();

  readonly data = signal<ReflectorDashboardData | null>(null);
  readonly error = signal<string | null>(null);
  readonly loading = signal<boolean>(true);

  /**
   * Resuelve la ruta del endpoint respetando <base href="/dashboard/">
   * cuando el binario Go embebe el SPA bajo BASE_PATH.
   */
  private resolveStatusUrl(): string {
    const baseHref = this.document.querySelector('base')?.getAttribute('href') ?? '/';
    const normalizedBase = baseHref.endsWith('/') ? baseHref.slice(0, -1) : baseHref;
    return `${normalizedBase}/api/reflector-status`;
  }

  /** Petición única normalizada */
  fetchStatusOnce(): Observable<ReflectorDashboardData | null> {
    return this.http.get<BackendStatusRaw>(this.statusUrl).pipe(
      map((raw) => this.mapBackendToModel(raw)),
      tap((mapped) => {
        this.data.set(mapped);
        this.error.set(null);
        this.loading.set(false);
      }),
      catchError(() => {
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
    return interval(periodMs).pipe(
      startWith(0),
      switchMap(() => this.fetchStatusOnce()),
      shareReplay({ bufferSize: 1, refCount: true }),
      takeUntilDestroyed(this.destroyRef)
    );
  }

  private mapBackendToModel(raw: BackendStatusRaw): ReflectorDashboardData {
    const ysfConnected: YsfStation[] = (raw.ysfConnectedList ?? []).map((item) => ({
      callsign: item.callsign,
      gateway: item.type,
      duration: item.since,
      status: 'IDLE'
    }));

    const ysfLastHeard: YsfActivity[] = (raw.ysfLastTx ?? []).map((tx) => ({
      time: tx.ago,
      callsign: tx.callsign,
      target: 'ALL',
      gateway: raw.header?.badge ?? 'YSF',
      duration: tx.duration,
      loss: '0.0%'
    }));

    const dmrLastHeard: DmrActivity[] = (raw.dmrLastTx ?? []).map((tx) => ({
      time: tx.ago,
      callsign: tx.callsign,
      dmrId: '—',
      talkgroup: 'TGIF',
      source: 'MMDVM_Bridge',
      duration: tx.duration
    }));

    return {
      header: raw.header,
      stats: {
        ysfConnectedCount: raw.stats?.ysfConnectedNow ?? ysfConnected.length,
        ysfTotalHeardToday: raw.stats?.ysfTx24h ?? ysfLastHeard.length,
        dmrActiveCount: raw.stats?.dmrTx24h ?? dmrLastHeard.length,
        dmrTotalHeardToday: raw.stats?.stations24h ?? 0
      },
      ysfConnected,
      ysfLastHeard,
      dmrLastHeard
    };
  }
}
