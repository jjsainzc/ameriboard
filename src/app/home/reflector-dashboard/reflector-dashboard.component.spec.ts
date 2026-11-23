import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { ReflectorDashboardComponent } from './reflector-dashboard.component';
import { ReflectorService } from '../../core/services/reflector.service';

describe('ReflectorDashboardComponent', () => {
  it('labels every populated table cell for the mobile card layout', async () => {
    await TestBed.configureTestingModule({
      imports: [ReflectorDashboardComponent],
      providers: [provideRouter([]), provideHttpClient(), provideHttpClientTesting()]
    }).compileComponents();
    const fixture = TestBed.createComponent(ReflectorDashboardComponent);
    fixture.detectChanges();
    const http = TestBed.inject(HttpTestingController);
    http.expectOne(request => request.url.endsWith('/api/profiles')).flush([]);
    http.expectOne(request => request.url.endsWith('/api/reflector-status')).flush({
      header: { title: 'Test', subtitle: '', badge: '', localBridgeStatus: '', lastUpdate: '' },
      dvSwitchBridges: [{ modeA: 'DMR', modeB: 'YSF' }, { modeA: 'D-Star', modeB: 'NXDN' }],
      stats: { ysfConnectedNow: 1, ysfTx24h: 1, dmrTx24h: 1, dvSwitchTx24h: 6, stations24h: 1 },
      ysfConnectedList: [{ callsign: 'TEST1', ip: '192.0.2.1', type: 'Gateway', since: '10:00' }],
      ysfLastTx: [{ callsign: 'TEST1', duration: '5s', ago: '1m' }],
      dmrLastTx: [{ callsign: 'TEST1', duration: '5s', ago: '1m' }],
      dvSwitchLastTx: [
        { rxTime: '11-23 05:00:55', mode: 'P25', callsign: 'TEST1', name: '—', target: 'TG 740', source: 'Net', duration: '2.7', loss: '—', ber: '—' },
        { rxTime: '11-23 04:59:10', mode: 'YSF', callsign: 'TEST2', name: '—', target: 'TG 74001', source: 'LNet', duration: '—', loss: '—', ber: '—' },
        { rxTime: '11-23 04:58:59', mode: 'YSF', callsign: 'TEST3', name: '—', target: 'ALL at HC1LNM', source: 'Net', duration: '0.6', loss: '0%', ber: '0.0%' },
        { rxTime: '11-23 04:58:13', mode: 'NXDN', callsign: 'TEST4', name: '—', target: 'TG 8465', source: 'Net', duration: '2.6', loss: '—', ber: '0.0%' },
        { rxTime: '11-23 04:57:56', mode: 'D-Star', callsign: 'TEST5', name: '—', target: 'CQCQCQ', source: 'Net', duration: '2.5', loss: '0%', ber: '0.0%' },
        { rxTime: '11-23 04:56:25', mode: 'DMR', callsign: 'TEST6', name: '—', target: 'TG 368', source: 'LNet', duration: '2.7', loss: '—', ber: '—' }
      ],
      echolinkAvailable: true,
      echolinkConnectedList: [{ number: '1', user: 'TEST1', attribute: 'T' }],
      echolinkLastHeard: [{ callsign: 'TEST2', disconnectedAt: '2026-11-23T09:00:00-05:00', duration: '3m 10s' }]
    });
    fixture.detectChanges();
    const tables = fixture.nativeElement.querySelectorAll('table') as NodeListOf<HTMLTableElement>;
    expect(tables).toHaveLength(6);
    expect(fixture.nativeElement.querySelector('.header-badges')).toBeNull();
    expect(Array.from(fixture.nativeElement.querySelectorAll('.bridge-badge') as NodeListOf<HTMLElement>,
      badge => badge.textContent?.trim())).toEqual([
      'Puente configurado: DMR ↔ YSF', 'Puente configurado: D-Star ↔ NXDN'
    ]);
    for (const table of tables) {
      const headings = Array.from(table.querySelectorAll('thead th'), th => th.textContent?.trim());
      const row = table.querySelector('tbody tr')!;
      const labels = Array.from(row.querySelectorAll('td, th'), cell => cell.getAttribute('data-label'));
      expect(labels).toEqual(headings);
    }
    const modeBadges = fixture.nativeElement.querySelectorAll('.dvswitch-table .mode-badge') as NodeListOf<HTMLElement>;
    expect(Array.from(modeBadges, badge => badge.textContent?.trim()))
      .toEqual(['P25', 'YSF', 'YSF', 'NXDN', 'D-Star', 'DMR']);
    expect(Array.from(modeBadges, badge => Array.from(badge.classList).find(name => name !== 'mode-badge')))
      .toEqual(['mode-p25', 'mode-ysf', 'mode-ysf', 'mode-nxdn', 'mode-d-star', 'mode-dmr']);
    expect(fixture.nativeElement.querySelector('.kpi-card.border-emerald .kpi-value').textContent.trim()).toBe('6');
    fixture.destroy();
    http.verify();
  });

  it('displays the IP of each connected YSF station', async () => {
    await TestBed.configureTestingModule({
      imports: [ReflectorDashboardComponent],
      providers: [provideRouter([]), provideHttpClient(), provideHttpClientTesting()]
    }).compileComponents();
    const fixture = TestBed.createComponent(ReflectorDashboardComponent);
    fixture.detectChanges();
    const http = TestBed.inject(HttpTestingController);
    http.expectOne((request) => request.url.endsWith('/api/profiles')).flush([]);
    http.expectOne((request) => request.url.endsWith('/api/reflector-status')).flush({
      header: { title: 'Test', subtitle: '', badge: '', localBridgeStatus: '', lastUpdate: '' },
      stats: { ysfConnectedNow: 2, ysfTx24h: 0, dmrTx24h: 0, stations24h: 1 },
      ysfConnectedList: [
        { callsign: 'TEST1', ip: '192.0.2.1', type: 'Gateway', since: '10:00' },
        { callsign: 'TEST1', ip: '192.0.2.2', type: 'Gateway', since: '10:00' }
      ]
    });
    fixture.detectChanges();
    const table = fixture.nativeElement.querySelector('table') as HTMLTableElement;
    expect(table.textContent).toContain('IP');
    expect(table.textContent).toContain('192.0.2.1');
    expect(table.textContent).toContain('192.0.2.2');
    expect(table.querySelectorAll('tbody tr')).toHaveLength(2);
    fixture.destroy();
    http.verify();
  });

  it('displays a connection error', async () => {
    await TestBed.configureTestingModule({
      imports: [ReflectorDashboardComponent],
      providers: [provideRouter([]), provideHttpClient(), provideHttpClientTesting()]
    }).compileComponents();
    const fixture = TestBed.createComponent(ReflectorDashboardComponent);
    fixture.detectChanges();
    const http = TestBed.inject(HttpTestingController);
    http.expectOne((request) => request.url.endsWith('/api/profiles')).flush([]);
    http.expectOne((request) => request.url.endsWith('/api/reflector-status'))
      .flush('error', { status: 500, statusText: 'Server Error' });
    fixture.detectChanges();
    expect(fixture.nativeElement.querySelector('[role="alert"]').textContent)
      .toContain('Sin conexión');
    fixture.destroy();
    http.verify();
  });

  it('keeps the local dashboard selectable when a remote site fails', async () => {
    await TestBed.configureTestingModule({
      imports: [ReflectorDashboardComponent],
      providers: [provideRouter([]), provideHttpClient(), provideHttpClientTesting()]
    }).compileComponents();
    const fixture = TestBed.createComponent(ReflectorDashboardComponent);
    fixture.detectChanges();
    const http = TestBed.inject(HttpTestingController);
    const status = {
      header: { title: 'Local', subtitle: '', badge: '', localBridgeStatus: '', lastUpdate: '' },
      stats: { ysfConnectedNow: 0, ysfTx24h: 0, dmrTx24h: 0, stations24h: 0 }
    };
    http.expectOne((request) => request.url.endsWith('/api/profiles'))
      .flush([{ id: 'remote', name: 'Remoto', visible: true }]);
    http.expectOne((request) => request.url.endsWith('/api/reflector-status')).flush(status);
    fixture.detectChanges();
    const select = fixture.nativeElement.querySelector('#site') as HTMLSelectElement;
    select.value = 'remote';
    select.dispatchEvent(new Event('change'));
    http.expectOne((request) => request.url.endsWith('/api/profiles/remote/status'))
      .flush('error', { status: 502, statusText: 'Bad Gateway' });
    fixture.detectChanges();
    expect(fixture.nativeElement.textContent).toContain('Sin conexión');
    expect(fixture.nativeElement.textContent).not.toContain('Local');
    select.value = '';
    select.dispatchEvent(new Event('change'));
    http.expectOne((request) => request.url.endsWith('/api/reflector-status')).flush(status);
    fixture.detectChanges();
    expect(fixture.nativeElement.textContent).toContain('Local');
    fixture.destroy();
    http.verify();
  });

  it('refreshes the selected site manually, including after an error', async () => {
    await TestBed.configureTestingModule({
      imports: [ReflectorDashboardComponent],
      providers: [provideRouter([]), provideHttpClient(), provideHttpClientTesting()]
    }).compileComponents();
    const fixture = TestBed.createComponent(ReflectorDashboardComponent);
    fixture.detectChanges();
    const http = TestBed.inject(HttpTestingController);
    http.expectOne(request => request.url.endsWith('/api/profiles'))
      .flush([{ id: 'remote', name: 'Remoto', visible: true }]);
    http.expectOne(request => request.url.endsWith('/api/reflector-status'))
      .flush('error', { status: 500, statusText: 'Error' });
    const select = fixture.nativeElement.querySelector('#site') as HTMLSelectElement;
    fixture.detectChanges();
    select.value = 'remote';
    select.dispatchEvent(new Event('change'));
    http.expectOne(request => request.url.endsWith('/api/profiles/remote/status'))
      .flush('error', { status: 502, statusText: 'Error' });
    fixture.detectChanges();
    const button = Array.from(fixture.nativeElement.querySelectorAll('button') as NodeListOf<HTMLButtonElement>)
      .find(element => element.textContent?.includes('Refrescar datos'));
    expect(button).toBeDefined();
    button!.click();
    http.expectOne(request => request.url.endsWith('/api/profiles/remote/status')).flush({
      header: { title: 'Remoto', subtitle: '', badge: '', localBridgeStatus: '', lastUpdate: '' },
      stats: { ysfConnectedNow: 0, ysfTx24h: 0, dmrTx24h: 0, stations24h: 0 }
    });
    fixture.detectChanges();
    expect(fixture.nativeElement.textContent).toContain('Remoto');
    expect(TestBed.inject(ReflectorService).error()).toBeNull();
    fixture.destroy();
    http.verify();
  });

  it('hides EchoLink tables when live EchoLink is unavailable, even with saved history', async () => {
    await TestBed.configureTestingModule({
      imports: [ReflectorDashboardComponent],
      providers: [provideRouter([]), provideHttpClient(), provideHttpClientTesting()]
    }).compileComponents();
    const fixture = TestBed.createComponent(ReflectorDashboardComponent);
    fixture.detectChanges();
    const http = TestBed.inject(HttpTestingController);
    http.expectOne(request => request.url.endsWith('/api/profiles')).flush([]);
    http.expectOne(request => request.url.endsWith('/api/reflector-status')).flush({
      header: { title: 'Local', subtitle: '', badge: '', localBridgeStatus: '', lastUpdate: '' },
      stats: { ysfConnectedNow: 0, ysfTx24h: 0, dmrTx24h: 0, stations24h: 0 },
      echolinkAvailable: false,
      echolinkLastHeard: [{ callsign: 'KQ4HPI-R', disconnectedAt: '2026-11-23T09:00:00-05:00', duration: '3m 10s' }]
    });
    fixture.detectChanges();

    expect(fixture.nativeElement.querySelector('#echolink-title')).toBeNull();
    expect(fixture.nativeElement.querySelector('#echolink-history-title')).toBeNull();
    fixture.destroy();
    http.verify();
  });

  it('hides all source cards and tables without alerts when sources are absent', async () => {
    await TestBed.configureTestingModule({
      imports: [ReflectorDashboardComponent],
      providers: [provideRouter([]), provideHttpClient(), provideHttpClientTesting()]
    }).compileComponents();
    const fixture = TestBed.createComponent(ReflectorDashboardComponent);
    fixture.detectChanges();
    const http = TestBed.inject(HttpTestingController);
    http.expectOne(request => request.url.endsWith('/api/profiles')).flush([]);
    http.expectOne(request => request.url.endsWith('/api/reflector-status')).flush({
      header: { title: 'Local', subtitle: '', badge: '', localBridgeStatus: '', lastUpdate: '' },
      stats: { ysfConnectedNow: 0, ysfTx24h: 0, dmrTx24h: 0, stations24h: 0 },
      ysfAvailable: false,
      ysfTrafficAvailable: false,
      dmrAvailable: false,
      echolinkAvailable: false,
      echolinkLastHeard: [],
      warnings: ['No se encontraron logs YSF recientes', 'No se encontraron logs DMR recientes', 'EchoLink no está disponible']
    });
    fixture.detectChanges();

    expect(fixture.nativeElement.querySelector('[role="alert"]')).toBeNull();
    expect(fixture.nativeElement.querySelector('.dash-header h1').textContent).toContain('Local');
    expect(fixture.nativeElement.querySelector('.kpi-grid')).toBeNull();
    expect(fixture.nativeElement.querySelectorAll('table')).toHaveLength(0);
    expect(fixture.nativeElement.querySelector('.bridge-badge')).toBeNull();
    fixture.destroy();
    http.verify();
  });

  it('shows only cards and tables supported by available sources', async () => {
    await TestBed.configureTestingModule({
      imports: [ReflectorDashboardComponent],
      providers: [provideRouter([]), provideHttpClient(), provideHttpClientTesting()]
    }).compileComponents();
    const fixture = TestBed.createComponent(ReflectorDashboardComponent);
    fixture.detectChanges();
    const http = TestBed.inject(HttpTestingController);
    const status = {
      header: { title: 'Local', subtitle: '', badge: '', localBridgeStatus: '', lastUpdate: '' },
      stats: { ysfConnectedNow: 1, ysfTx24h: 2, dmrTx24h: 3, stations24h: 4 }
    };
    http.expectOne(request => request.url.endsWith('/api/profiles')).flush([]);
    http.expectOne(request => request.url.endsWith('/api/reflector-status')).flush({
      ...status, ysfAvailable: false, ysfTrafficAvailable: true, dmrAvailable: true, echolinkAvailable: false
    });
    fixture.detectChanges();
    expect(fixture.nativeElement.querySelector('.border-cyan')).toBeNull();
    expect(fixture.nativeElement.querySelector('.border-purple')).toBeNull();
    expect(fixture.nativeElement.querySelector('.border-blue')).not.toBeNull();
    expect(fixture.nativeElement.querySelector('.border-emerald')).not.toBeNull();
    expect(fixture.nativeElement.querySelector('.ysf-last-heard')).not.toBeNull();
    expect(fixture.nativeElement.querySelector('.dvswitch-table')).not.toBeNull();
    expect(fixture.nativeElement.querySelector('#echolink-title')).toBeNull();

    TestBed.inject(ReflectorService).fetchStatusOnce().subscribe();
    http.expectOne(request => request.url.endsWith('/api/reflector-status')).flush({
      ...status, ysfAvailable: true, ysfTrafficAvailable: true, dmrAvailable: false, echolinkAvailable: true
    });
    fixture.detectChanges();
    expect(fixture.nativeElement.querySelector('.border-cyan')).not.toBeNull();
    expect(fixture.nativeElement.querySelector('.border-blue')).not.toBeNull();
    expect(fixture.nativeElement.querySelector('.border-emerald')).toBeNull();
    expect(fixture.nativeElement.querySelector('.border-purple')).toBeNull();
    expect(fixture.nativeElement.querySelector('.dvswitch-table')).toBeNull();
    expect(fixture.nativeElement.querySelector('#echolink-title')).not.toBeNull();
    expect(fixture.nativeElement.querySelector('#echolink-history-title')).not.toBeNull();
    expect(fixture.nativeElement.querySelector('[role="alert"]')).toBeNull();
    fixture.destroy();
    http.verify();
  });

  it('hides unavailable EchoLink and shows its attribute legend when available', async () => {
    await TestBed.configureTestingModule({
      imports: [ReflectorDashboardComponent],
      providers: [provideRouter([]), provideHttpClient(), provideHttpClientTesting()]
    }).compileComponents();
    const fixture = TestBed.createComponent(ReflectorDashboardComponent);
    fixture.detectChanges();
    const http = TestBed.inject(HttpTestingController);
    const status = {
      header: { title: 'Local', subtitle: '', badge: '', localBridgeStatus: '', lastUpdate: '' },
      stats: { ysfConnectedNow: 0, ysfTx24h: 0, dmrTx24h: 0, stations24h: 0 }
    };
    http.expectOne(request => request.url.endsWith('/api/profiles')).flush([]);
    http.expectOne(request => request.url.endsWith('/api/reflector-status'))
      .flush({ ...status, echolinkAvailable: false });
    fixture.detectChanges();
    expect(fixture.nativeElement.querySelector('#echolink-title')).toBeNull();

    TestBed.inject(ReflectorService).fetchStatusOnce().subscribe();
    http.expectOne(request => request.url.endsWith('/api/reflector-status')).flush({
      ...status, echolinkAvailable: true,
      echolinkConnectedList: [{ number: '1', user: 'TEST', attribute: 'T' }]
    });
    fixture.detectChanges();
    expect(fixture.nativeElement.querySelector('#echolink-title').textContent).toContain('EchoLink');
    const dialog = fixture.nativeElement.querySelector('dialog') as HTMLDialogElement;
    const show = vi.fn();
    const close = vi.fn();
    dialog.showModal = show;
    dialog.close = close;
    fixture.nativeElement.querySelector('[aria-haspopup="dialog"]').click();
    expect(show).toHaveBeenCalledOnce();
    expect(Array.from(dialog.querySelectorAll('tbody th')).map(cell => cell.textContent?.trim()))
      .toEqual(['A', 'a', 'B', 'C', 'c', 'F', 'f', 'K', 'L', 'M', 'm', 'P', 'R', 'S', 'T', 'u', 'x', '*', '0', '1', '!']);
    dialog.querySelector('button')?.click();
    expect(close).toHaveBeenCalledOnce();
    fixture.destroy();
    http.verify();
  });
});
