import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { SiteAdminComponent } from './site-admin.component';

describe('SiteAdminComponent', () => {
  it('creates the first administrator and opens the administration page', async () => {
    await TestBed.configureTestingModule({
      imports: [SiteAdminComponent],
      providers: [provideRouter([]), provideHttpClient(), provideHttpClientTesting()]
    }).compileComponents();
    const fixture = TestBed.createComponent(SiteAdminComponent);
    const component = fixture.componentInstance;
    const http = TestBed.inject(HttpTestingController);
    fixture.detectChanges();
    http.expectOne(request => request.url.endsWith('/api/admin/profiles'))
      .flush({}, { status: 401, statusText: 'Unauthorized' });
    http.expectOne(request => request.url.endsWith('/api/admin/setup')).flush({ required: true });
    fixture.detectChanges();
    expect(fixture.nativeElement.textContent).toContain('Crear administrador');
    component.setupForm.setValue({ password: '12345678', confirm: 'different password' });
    component.setup();
    http.expectNone(request => request.method === 'POST');
    const passwordInput = fixture.nativeElement.querySelector('#setup-password') as HTMLInputElement;
    expect(passwordInput.type).toBe('password');
    (fixture.nativeElement.querySelector('input[type="checkbox"]') as HTMLInputElement).click();
    fixture.detectChanges();
    expect(passwordInput.type).toBe('text');
    component.setupForm.patchValue({ confirm: '12345678' });
    component.setup();
    const create = http.expectOne(request => request.method === 'POST' && request.url.endsWith('/api/admin/setup'));
    expect(create.request.body).toEqual({ password: '12345678' });
    create.flush(null, { status: 204, statusText: 'No Content' });
    http.expectOne(request => request.method === 'GET' && request.url.endsWith('/api/admin/profiles')).flush([]);
    http.expectOne(request => request.method === 'GET' && request.url.endsWith('/api/admin/settings'))
      .flush({ title: 'Ameriboard', subtitle: 'Panel de monitoreo de YSF, EchoLink y DVSwitch', dvSwitchTitle: 'Actividad digital vía DVSwitch' });
    fixture.detectChanges();
    expect(fixture.nativeElement.textContent).toContain('Administrador creado');
    expect(component.setupForm.value.password).toBe('');
    expect(component.mainSettingsForm.value.title).toBe('Ameriboard');
    fixture.destroy();
    http.verify();
  });

  it('creates a hidden site and preserves its password when editing', async () => {
    await TestBed.configureTestingModule({
      imports: [SiteAdminComponent],
      providers: [provideRouter([]), provideHttpClient(), provideHttpClientTesting()]
    }).compileComponents();
    const fixture = TestBed.createComponent(SiteAdminComponent);
    const component = fixture.componentInstance;
    const http = TestBed.inject(HttpTestingController);
    fixture.detectChanges();
    http.expectOne(request => request.url.endsWith('/api/admin/profiles')).flush([]);
    http.expectOne(request => request.url.endsWith('/api/admin/settings'))
      .flush({
        title: 'Título original', subtitle: 'Subtítulo original', dvSwitchTitle: 'Actividad digital vía DVSwitch'
      });
    fixture.detectChanges();
    component.mainSettingsForm.setValue({
      title: 'Título actualizado', subtitle: 'Nuevo subtítulo', dvSwitchTitle: 'Actividad local', nxdnTitle: 'NXDN local'
    });
    component.saveMainSettings();
    const settingsUpdate = http.expectOne(request => request.method === 'PUT' && request.url.endsWith('/api/admin/settings'));
    expect(settingsUpdate.request.body).toEqual({
      title: 'Título actualizado', subtitle: 'Nuevo subtítulo', dvSwitchTitle: 'Actividad local', nxdnTitle: 'NXDN local'
    });
    expect(settingsUpdate.request.headers.get('X-Ameriboard-Request')).toBe('1');
    settingsUpdate.flush({
      title: 'Título actualizado', subtitle: 'Nuevo subtítulo', dvSwitchTitle: 'Actividad local', nxdnTitle: 'NXDN local'
    });
    fixture.detectChanges();
    expect(fixture.nativeElement.textContent).toContain('Configuración principal guardada');
    expect(fixture.nativeElement.querySelector('#site-name')).toBeNull();
    const addButton = Array.from(fixture.nativeElement.querySelectorAll('button') as NodeListOf<HTMLButtonElement>)
      .find(button => button.textContent?.trim() === 'Agregar sitio');
    expect(addButton).toBeDefined();
    addButton!.click();
    fixture.detectChanges();
    expect(fixture.nativeElement.querySelector('#site-name')).not.toBeNull();
    component.siteForm.setValue({
      name: 'Sitio', url: 'https://example.org/ameriboard', password: 'a secure password', visible: false,
      title: 'Mi reflector', subtitle: '', badge: 'Zona', dvSwitchTitle: 'Actividad remota', nxdnTitle: 'NXDN remoto'
    });
    component.save();
    const create = http.expectOne(request => request.method === 'POST' && request.url.endsWith('/api/admin/profiles'));
    expect(create.request.body.visible).toBe(false);
    expect(create.request.body.password).toBe('a secure password');
    expect(create.request.body.header).toEqual({
      title: 'Mi reflector', subtitle: '', badge: 'Zona', dvSwitchTitle: 'Actividad remota', nxdnTitle: 'NXDN remoto'
    });
    expect(create.request.headers.get('X-Ameriboard-Request')).toBe('1');
    create.flush({ id: '1', name: 'Sitio', url: 'https://example.org/ameriboard', visible: false });
    http.expectOne(request => request.method === 'GET' && request.url.endsWith('/api/admin/profiles'))
      .flush([{ id: '1', name: 'Sitio', url: 'https://example.org/ameriboard', visible: false,
        header: { title: 'Mi reflector', subtitle: '', badge: 'Zona', dvSwitchTitle: 'Actividad remota', nxdnTitle: 'NXDN remoto' } }]);
    http.expectOne(request => request.method === 'GET' && request.url.endsWith('/api/admin/settings'))
      .flush({ title: 'Título actualizado', subtitle: 'Nuevo subtítulo', dvSwitchTitle: 'Actividad local' });
    fixture.detectChanges();
    expect(fixture.nativeElement.querySelector('#site-name')).toBeNull();
    const editButton = Array.from(fixture.nativeElement.querySelectorAll('button') as NodeListOf<HTMLButtonElement>)
      .find(button => button.textContent?.trim() === 'Editar');
    expect(editButton).toBeDefined();
    editButton!.click();
    fixture.detectChanges();
    expect(fixture.nativeElement.querySelector('#site-name')).not.toBeNull();
    expect(component.siteForm.value.password).toBe('');
    expect(component.siteForm.value.title).toBe('Mi reflector');
    expect(component.siteForm.value.dvSwitchTitle).toBe('Actividad remota');
    expect(component.siteForm.value.nxdnTitle).toBe('NXDN remoto');
    component.save();
    const update = http.expectOne(request => request.method === 'PUT' && request.url.endsWith('/api/admin/profiles/1'));
    expect(update.request.body.password).toBe('');
    expect(update.request.body.header.badge).toBe('Zona');
    expect(update.request.body.header.dvSwitchTitle).toBe('Actividad remota');
    expect(update.request.body.header.nxdnTitle).toBe('NXDN remoto');
    update.flush('error', { status: 500, statusText: 'Error' });
    fixture.detectChanges();
    expect(fixture.nativeElement.querySelector('#site-name')).not.toBeNull();
    component.save();
    http.expectOne(request => request.method === 'PUT' && request.url.endsWith('/api/admin/profiles/1')).flush({});
    http.expectOne(request => request.method === 'GET' && request.url.endsWith('/api/admin/profiles')).flush([]);
    http.expectOne(request => request.method === 'GET' && request.url.endsWith('/api/admin/settings'))
      .flush({ title: 'Título actualizado', subtitle: 'Nuevo subtítulo', dvSwitchTitle: 'Actividad local' });
    fixture.detectChanges();
    expect(fixture.nativeElement.querySelector('#site-name')).toBeNull();
    fixture.destroy();
    http.verify();
  });
});
