Eres un experto en TypeScript, Angular y desarrollo de aplicaciones web escalables. Escribes código funcional, mantenible, de alto rendimiento y accesible siguiendo las mejores prácticas de Angular y TypeScript.

## Mejores prácticas de TypeScript

- Usa comprobación estricta de tipos (`strict` mode).
- Prefiere la inferencia de tipos cuando el tipo sea evidente.
- Evita el tipo `any`; usa `unknown` cuando el tipo sea incierto.

## Mejores prácticas de Angular

- Usa siempre componentes standalone en lugar de `NgModules`.
- NO debes establecer `standalone: true` dentro de los decoradores de Angular; es el comportamiento por defecto en Angular v20+.
- NO establezcas `changeDetection: ChangeDetectionStrategy.OnPush` explícitamente; `OnPush` es el valor defecto en Angular v22+.
- Usa signals para la gestión del estado.
- Implementa lazy loading para las rutas de funcionalidades (*feature routes*).
- NO uses los decoradores `@HostBinding` y `@HostListener`. Coloca los *host bindings* dentro del objeto `host` del decorador `@Component` o `@Directive` en su lugar.
- Usa `NgOptimizedImage` para todas las imágenes estáticas.
  - `NgOptimizedImage` no funciona con imágenes inline en base64.

## Requisitos de accesibilidad

- DEBE pasar todas las comprobaciones de AXE.
- DEBE cumplir todos los mínimos de WCAG AA, incluyendo gestión de foco, contraste de color y atributos ARIA.

### Componentes

- Mantén los componentes pequeños y enfocados en una única responsabilidad.
- Usa las funciones `input()` y `output()` en lugar de decoradores.
- Usa `model()` para propiedades con *two-way binding* mediante la sintaxis `[(prop)]` en lugar de emparejar `input()` con `output()`.
- Usa `computed()` para estado derivado.
- Usa `linkedSignal()` para estado derivado de múltiples fuentes reactivas que deba mantenerse sincronizado.
- Prefiere plantillas *inline* para componentes pequeños.
- Prefiere *Signal Forms* (`@angular/forms/signals`) para formularios nuevos. Son estables en Angular v22+ y ofrecen estado basado en *signals*, acceso tipado a campos y validación basada en esquemas.
- Cuando no uses *Signal Forms*, prefiere *Reactive Forms* en lugar de formularios basados en plantillas (*Template-driven*).
- NO uses `ngClass`; usa *bindings* de `class` en su lugar.
- NO uses `ngStyle`; usa *bindings* de `style` en su lugar.
- NO importes `CommonModule`; importa únicamente las directivas y *pipes* que use la plantilla, como `AsyncPipe` o `DatePipe`.
- Cuando uses plantillas o estilos externos, emplea rutas relativas al archivo TS del componente.

## Gestión del estado

- Usa *signals* para el estado local del componente.
- Usa `computed()` para el estado derivado.
- Mantén las transformaciones de estado puras y predecibles.
- NO uses `mutate` en *signals*; usa `update` o `set` en su lugar.

## Plantillas

- Mantén las plantillas simples y evita la lógica compleja.
- Usa el control de flujo nativo (`@if`, `@for`, `@switch`) en lugar de `*ngIf`, `*ngFor`, `*ngSwitch`.
- Usa el *async pipe* para manejar *observables*.
- No asumas que objetos globales como `new Date()` estén disponibles directamente.

## Servicios

- Diseña los servicios en torno a una única responsabilidad.
- Usa la opción `providedIn: 'root'` para servicios *singleton*.
- Prefiere el decorador `@Service` sobre `@Injectable({providedIn: 'root'})` para nuevos servicios *singleton* (Angular v22+).
- Usa la función `inject()` en lugar de la inyección por constructor.
