# Ameriboard

- [x] 1. Los datos titulo y subtitulo del sitio principal no tienen edicion.
- [x] 2. Si falta alguna fuente (YSF, EchoLink o DVSwitch), ocultar sus tablas y tarjetas sin mostrar errores ni avisos por su ausencia.
- [ ] 3. La tabla del logs de Echolink esta muy ancha y solo va a contener esos atributos, entonces reducir su ancho, la maxima cantidad de columnas de los atributos pudiera se la suma de las latras de sus atributos, y estos nunca van a set todos juntos. Pendiente de validar en un servidor externo.
- [x] 4. La tabla del DVSwitch no tiene que tener un puente, es actividad por modalidad, y esta columna puede agregarse a la tabla y tambien a la tarjeta y la red no tiene que ser solamente TGIF, se puede poner en el perfil del sitio si se sdesa poner un titulo personalizado a este log, ejemplo de 4 modalidades para la tabla de DVSwitch.
```
I: 2026-11-23 09:56:20.998 DMR, TAG_SET_INFO: src=7400203 rpt=740020306 dst=9 slot=2 cc=1 metadata=HC1LNM
M: 2026-11-23 09:56:20.998 DMR, Remote CMD: txTg=368
M: 2026-11-23 09:56:20.998 DMR, Tune: id=7400203, tg=368, mode=Group
M: 2026-11-23 09:56:25.190 DMR, TX state = ON
I: 2026-11-23 09:56:25.190 DMR, Begin TX: src=7400203 rpt=740020306 dst=368 slot=2 cc=1 metadata=HC1LNM
M: 2026-11-23 09:56:27.804 DMR, TX state = OFF, DMR frame count was 46 frames

I: 2026-11-23 09:58:58.289 NXDN, TAG_SET_INFO: src=6726 rpt=740020306 dst=8465 slot=2 cc=1 metadata=HC1LNM
I: 2026-11-23 09:58:13.697 NXDN, Begin TX: src=7400203 rpt=740020306 dst=8465 slot=2 cc=1 metadata=HC1LNM
I: 2026-11-23 09:58:13.697 NXDN, (7400203, 8465) -> (6726, 8465)
M: 2026-11-23 09:58:13.697 NXDN, received RF header from HC1LNM to TG 8465
M: 2026-11-23 09:58:16.245 NXDN, TX state = OFF
M: 2026-11-23 09:58:16.245 NXDN, received RF end of transmission, 2.6 seconds, BER: 0.0%

I: 2026-11-23 10:00:50.835 YSF, TAG_SET_INFO: src=7400203 rpt=740020306 dst=74001 slot=2 cc=1 metadata=HC1LNM
M: 2026-11-23 09:59:10.164 YSF, TX state = ON
I: 2026-11-23 09:59:10.164 YSF, Begin TX: src=7400203 rpt=740020306 dst=74001 slot=2 cc=1 metadata=HC1LNM
I: 2026-11-23 09:59:10.266 YSF, Narrow transmit (72 bit)
M: 2026-11-23 09:59:12.172 YSF, TX state = OFF

I: 2026-11-23 10:00:50.835 YSF, TAG_SET_INFO: src=7400203 rpt=740020306 dst=74001 slot=2 cc=1 metadata=HC1LNM
M: 2026-11-23 10:00:50.835 P25, Remote CMD: txTg=740
M: 2026-11-23 10:00:55.589 P25, TX state = ON
I: 2026-11-23 10:00:55.589 P25, Begin TX: src=7400203 rpt=740020306 dst=740 slot=2 cc=1 metadata=HC1LNM
M: 2026-11-23 10:00:58.386 P25, TX state = OFF
```

