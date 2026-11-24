# Ameriboard

- [x] 1. Los datos titulo y subtitulo del sitio principal no tienen edicion.
- [x] 2. Si falta alguna fuente (YSF, EchoLink o DVSwitch), ocultar sus tablas y tarjetas sin mostrar errores ni avisos por su ausencia.
- [x] 3. La tabla del logs de Echolink esta muy ancha y solo va a contener esos atributos, entonces reducir su ancho, la maxima cantidad de columnas de los atributos pudiera se la suma de las latras de sus atributos, y estos nunca van a set todos juntos. Pendiente de validar en un servidor externo.
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

- [x] 5. Siguiendo las instrucciones de este manual /home/hc1lnm/Documents/AngularPro/ameriboard/doc/tlbcmd-commands.txt implementar en la tabla de echolink, para administradores, en la misma tabla de echolink, crear una columna de acciones con iconos que se activan o se desactivan por ejemmplo para las opciones mutear/desmutear, para desconectar una X, para las demas opciones poner un menu desplegable que permita las operaciones usando ventanas flotantes, cuando sean necesario, en principio implementar connect,disconnect, ban, mute, message

- [x] 6. Poner en el encabezado de la tabla de YSF el nombre, numero y puerto activo del reflector que se toma en `/etc/YSFReflector.ini` esta es una de las versiones de ese reflector., normalmente la conexiones que se hacen localmente son puentes.
Como no hay una instalacion aca de ese file, dejo un ejemplo de donde se debe tomar los datos:
```
[Info]
Id=31645
Name=US EAGLES
Description=Florida Eagles
```

- [x] 7. Tenemos otra version de reflector YSF hecha con python, los files estan en `/opt/pysfreflector`, ahi esta el nombre del service que se deberia buscar para ver si esta instalado, y tomar del ini, los datos para hacer otro juego de ventanas de ese reflector, los logs estan definidos en el ini que tambien el service debe indicar el path, pero como no esta instalado en este server hay un ejemplo de log en `/opt/pysfreflector/example-log`, de igual forma si este reflector no esta instalado ni activo no se muestran las pantallas, otra forma de determinarloe es asi:
```
root@ubuntu-s-1vcpu-512mb-10gb-nyc1-01:~# netstat -tulpn | grep 4200
udp        0      0 0.0.0.0:42000           0.0.0.0:*                           13792/./YSFReflecto
udp        0      0 0.0.0.0:42001           0.0.0.0:*                           160708/python3
```
generalmente los puertos asignados para estos reflectores de YSF son del 42000 - 4200X

- [x] 8. Hacer las secciones para agrupar las tablas de forma colapsable como las que tiene el Amerimon.

- [ ] 9. Incorporar las tarjetas y ultimos escuchados para un reflector NXDN, el ini esta en `/etc/NXDNReflector.ini` los logs estan en `/var/log/NXDNReflector-YYYY-MM-DD.log` o `/var/log/NXDNReflector.log`
Ejemplo del log
```
M: 2026-11-24 00:01:56.434 Currently linked repeaters:
M: 2026-11-24 00:01:56.435     N6JET     : 5.161.215.224:3980 0/120
M: 2026-11-24 00:01:56.435     KJ4GNU    : 108.86.168.93:57910 3/120
M: 2026-11-24 00:01:56.435     HI3ACS    : 181.36.184.30:57910 3/120
M: 2026-11-24 00:01:56.435     KQ4HPI    : 57.135.192.137:57910 3/120
M: 2026-11-24 00:01:56.435     N2AST     : 148.74.218.1:57910 2/120
M: 2026-11-24 00:03:56.439 Currently linked repeaters:
M: 2026-11-24 00:03:56.439     N6JET     : 5.161.215.224:3980 0/120
M: 2026-11-24 00:03:56.439     KJ4GNU    : 108.86.168.93:57910 3/120
M: 2026-11-24 00:03:56.439     HI3ACS    : 181.36.184.30:57910 2/120
M: 2026-11-24 00:03:56.439     KQ4HPI    : 57.135.192.137:57910 2/120
M: 2026-11-24 00:03:56.439     N2AST     : 148.74.218.1:57910 2/120
M: 2026-11-24 00:05:06.975 Adding KQ4HPI     (57.135.192.137:57278)
M: 2026-11-24 00:05:56.449 Currently linked repeaters:
M: 2026-11-24 00:05:56.449     N6JET     : 5.161.215.224:3980 0/120
M: 2026-11-24 00:05:56.449     KJ4GNU    : 108.86.168.93:57910 3/120
M: 2026-11-24 00:05:56.449     HI3ACS    : 181.36.184.30:57910 2/120
M: 2026-11-24 00:05:56.449     KQ4HPI    : 57.135.192.137:57910 97/120
M: 2026-11-24 00:05:56.449     N2AST     : 148.74.218.1:57910 2/120
M: 2026-11-24 00:05:56.449     KQ4HPI    : 57.135.192.137:57278 4/120
M: 2026-11-24 00:06:18.702 Removing KQ4HPI     (57.135.192.137:57910) disappeared
``` 
Diferenciar las  tarjetas de reflectores con diferentes colores o algo similar entre YSF - NXDN - ECHOLINK y DVSwitch

