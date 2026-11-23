import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { provideRouter, Router } from '@angular/router';
import { App } from './app';
import { routes } from './app.routes';

describe('App', () => {
  it('loads the dashboard route', async () => {
    await TestBed.configureTestingModule({
      imports: [App],
      providers: [provideRouter(routes), provideHttpClient(), provideHttpClientTesting()]
    }).compileComponents();
    const fixture = TestBed.createComponent(App);
    fixture.detectChanges();
    TestBed.inject(HttpTestingController).expectOne((request) => request.url.endsWith('/api/version'))
      .flush({ version: '1.2.3' });
    await TestBed.inject(Router).navigateByUrl('/');
    fixture.detectChanges();
    const footer = fixture.nativeElement.querySelector('footer') as HTMLElement;
    expect(footer.textContent).toContain('Versión 1.2.3');
    expect(footer.textContent).toContain('Powered by');
    expect(footer.textContent).toContain('Angular Framework');
    expect(footer.textContent).toContain('Go Language');
    expect(fixture.nativeElement.querySelector('app-reflector-dashboard')).toBeTruthy();
    TestBed.inject(HttpTestingController).expectOne((request) => request.url.endsWith('/api/reflector-status'))
      .flush({ header: { title: 'Test' }, stats: {} });
    TestBed.inject(HttpTestingController).expectOne((request) => request.url.endsWith('/api/profiles')).flush([]);
    fixture.destroy();
  });
});
