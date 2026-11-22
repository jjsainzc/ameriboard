import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { ReflectorDashboardComponent } from './reflector-dashboard.component';
import { ReflectorService } from '../../core/services/reflector.service';

describe('ReflectorDashboardComponent', () => {
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
