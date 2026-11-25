import { DOCUMENT } from '@angular/common';
import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { DEFAULT_NXDN_TITLE, normalizeNXDNTitle, ReflectorService } from './reflector.service';

describe('ReflectorService', () => {
  let base: HTMLBaseElement;
  let http: HttpTestingController;

  beforeEach(() => {
    TestBed.configureTestingModule({ providers: [provideHttpClient(), provideHttpClientTesting()] });
    const document = TestBed.inject(DOCUMENT);
    base = document.createElement('base');
    base.setAttribute('href', '/ameriboard/');
    document.head.prepend(base);
    http = TestBed.inject(HttpTestingController);
  });

  afterEach(() => {
    base.remove();
    http.verify();
  });

  it('updates the old NXDN default while preserving a custom connected title', () => {
    expect(normalizeNXDNTitle('Últimos escuchados NXDN')).toBe(DEFAULT_NXDN_TITLE);
    expect(normalizeNXDNTitle('Mi reflector NXDN')).toBe('Mi reflector NXDN');
  });

  it('requests the configured base path and maps the backend response', () => {
    const service = TestBed.inject(ReflectorService);
    service.fetchStatusOnce().subscribe();
    http.expectOne('/ameriboard/api/reflector-status').flush({
      header: { title: 'Test', subtitle: '', badge: '', localBridgeStatus: '', lastUpdate: '' },
      dvSwitchTitle: 'Actividad personalizada',
      dvSwitchBridges: [{ modeA: 'DMR', modeB: 'YSF' }],
      stats: { ysfConnectedNow: 1, ysfTx24h: 2, dmrTx24h: 3, dvSwitchTx24h: 7, stations24h: 4 },
      ysfConnectedList: [
        { callsign: 'TEST1', ip: '192.0.2.1', type: 'Gateway', since: '10:00' },
        { callsign: 'LOCAL', ip: '127.0.0.1', type: 'Puente', since: '10:00' }
      ],
      pysfReflector: {
        available: true, connected: [
          { callsign: 'PYSF1', ip: '192.0.2.2', type: 'Estación o gateway', since: '10:00' },
          { callsign: 'PYSF2', ip: '127.0.0.2', type: '', since: '10:00' }
        ]
      },
      nxdnReflector: {
        available: true, connected: [
          { callsign: 'NXDN1', ip: '192.0.2.1:1234', type: 'Estación o gateway', since: '10:00' },
          { callsign: 'NXDN2', ip: '127.0.0.1:1234', type: 'Puente', since: '10:00' }
        ]
      },
      ysfLastTx: [{ callsign: 'TEST1', duration: '5s', ago: 'hace 1m' }],
      dmrLastTx: [{ callsign: 'DMR1', duration: '10s', ago: 'hace 2m' }, { callsign: 'DMR1', duration: '8s', ago: 'hace 3m' }],
      dvSwitchLastTx: [{
        rxTime: '11-23 09:58:16', mode: 'NXDN', callsign: 'N0CALL', name: '—', target: 'TG 8465',
        source: 'Net', duration: '2.6', loss: '—', ber: '0.0%'
      }, {
        rxTime: '23/11/2026 09:58:17', mode: 'DMR', callsign: 'TEST1', name: '—', target: 'TG 9',
        source: 'Net', duration: '1.0', loss: '—', ber: '—'
      }],
      echolinkConnectedList: [{ number: '1', user: 'W2LRC-R', attribute: 'T' }],
      echolinkLastHeard: [{ callsign: 'KQ4HPI-R', disconnectedAt: '2026-11-23T09:00:00-05:00', duration: '2m 5s' }],
      echolinkUptime: 'tlbcmd Version 0.56\r\n0\r\nServer Uptime: 10 days'
    });
    expect(service.data()?.stats).toEqual({
      ysfConnectedCount: 1, ysfTx24h: 2, dmrTx24h: 3, dvSwitchTx24h: 7, stations24h: 4
    });
    expect(service.data()?.dvSwitchTitle).toBe('Actividad personalizada');
    expect(service.data()?.nxdnReflector.available).toBe(true);
    expect(service.data()?.nxdnTitle).toBe('Estaciones o gateways conectados NXDN');
    expect(service.data()?.dvSwitchBridges).toEqual([{ modeA: 'DMR', modeB: 'YSF' }]);
    expect(service.data()?.ysfConnected[0]).toEqual({ callsign: 'TEST1', ip: '192.0.2.1', gateway: 'Estación', since: '10:00' });
    expect(service.data()?.ysfConnected[1].gateway).toBe('Puente');
    expect(service.data()?.pysfReflector.connected.map(station => station.gateway)).toEqual(['Estación', 'Puente']);
    expect(service.data()?.nxdnReflector.connected.map(station => station.gateway)).toEqual(['Estación', 'Puente']);
    expect(service.data()?.ysfLastHeard[0].loss).toBe('—');
    expect(service.data()?.dmrLastHeard).toEqual([
      { time: 'hace 2m', callsign: 'DMR1', dmrId: '—', talkgroup: '—', source: '—', duration: '10s' }
    ]);
    expect(service.data()?.dvSwitchLastHeard).toEqual([
      {
        rxTime: '23/11 09:58:16', mode: 'NXDN', callsign: 'N0CALL', name: '—', target: 'TG 8465',
        source: 'Net', duration: '2.6', loss: '—', ber: '0.0%'
      },
      {
        rxTime: '23/11/2026 09:58:17', mode: 'DMR', callsign: 'TEST1', name: '—', target: 'TG 9',
        source: 'Net', duration: '1.0', loss: '—', ber: '—'
      }
    ]);
    expect(service.data()?.echolinkConnected).toEqual([{ number: '1', user: 'W2LRC-R', attribute: 'T' }]);
    expect(service.data()?.echolinkLastHeard).toEqual([
      { callsign: 'KQ4HPI-R', disconnectedAt: '2026-11-23T09:00:00-05:00', duration: '2m 5s' }
    ]);
    expect(service.data()?.echolinkUptime).toBe('tlbcmd Version 0.56\r\nServer Uptime: 10 days');
    expect(service.error()).toBeNull();
  });

  it('reports an invalid response instead of showing success', () => {
    const service = TestBed.inject(ReflectorService);
    service.fetchStatusOnce().subscribe();
    http.expectOne('/ameriboard/api/reflector-status').flush({ status: 'ok' });
    expect(service.error()).toBeTruthy();
    expect(service.data()).toBeNull();
    expect(service.loading()).toBe(false);
  });
});
