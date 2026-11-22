import { Injectable, inject, signal, DestroyRef } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { DOCUMENT } from '@angular/common';
import { Observable, interval, switchMap, startWith, catchError, of, tap, map, shareReplay } from 'rxjs';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { ReflectorDashboardData, YsfStation, YsfActivity, DmrActivity } from '../models/reflector.model';

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
    stations24h: number;
  };
  ysfLastTx?: Array<{ callsign: string; duration: string; ago: string }> | null;
  dmrLastTx?: Array<{ callsign: string; duration: string; ago: string }> | null;
  ysfConnectedList?: Array<{ callsign: string; ip: string; type: string; since: string }> | null;
  combined24h?: Array<{
    time: string;
    mode: string;
    callsign: string;
    duration: string;
    detail: string;
  }> | null;
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
   * Resuelve la ruta del endpoint respetando <base href="/ameriboard/">
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
      ip: item.ip,
      gateway: item.type,
      since: item.since
    }));

    const ysfLastHeard: YsfActivity[] = (raw.ysfLastTx ?? []).map((tx) => ({
      time: tx.ago,
      callsign: tx.callsign,
      target: '—',
      gateway: '—',
      duration: tx.duration,
      loss: '—'
    }));

    const dmrLastHeard: DmrActivity[] = (raw.dmrLastTx ?? []).map((tx) => ({
      time: tx.ago,
      callsign: tx.callsign,
      dmrId: '—',
      talkgroup: '—',
      source: '—',
      duration: tx.duration
    }));

    return {
      header: raw.header,
      stats: {
        ysfConnectedCount: raw.stats.ysfConnectedNow,
        ysfTx24h: raw.stats.ysfTx24h,
        dmrTx24h: raw.stats.dmrTx24h,
        stations24h: raw.stats.stations24h
      },
      ysfConnected,
      ysfLastHeard,
      dmrLastHeard
    };
  }
}
