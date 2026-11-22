import { Routes } from '@angular/router';

export const routes: Routes = [
  {
    path: '',
    loadComponent: () => import('./home/reflector-dashboard/reflector-dashboard.component')
      .then((m) => m.ReflectorDashboardComponent)
  },
  {
    path: 'admin',
    loadComponent: () => import('./home/site-admin/site-admin.component')
      .then((m) => m.SiteAdminComponent)
  }
];
