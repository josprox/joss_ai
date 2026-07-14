# joss_ai

Plugin oficial para habilitar el uso de Inteligencia Artificial (`AI` y `ChatClient`) en el lenguaje Joss.

## Instalación

```bash
joss pub add joss_ai
```

## Uso

```joss
use joss_ai;

$client = AI::client()->user("Hola, ¿cómo estás?")->call();
print($client);
```
