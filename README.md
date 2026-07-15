# joss_ai 2.0

`joss_ai` conecta Joss con proveedores de chat compatibles con OpenAI, Groq y Gemini. Es un paquete JP v2 autocontenido: incluye la API Joss, su indice publico para IntelliSense y sidecars nativos para Windows, Linux y macOS (amd64 y arm64).

El proyecto consumidor solo necesita Joss 3.6.0 o posterior. No necesita Go, Python, Java ni la herramienta de compilacion del plugin, y no se usa `use`.

## Instalacion

```bash
joss pub add joss_ai 2.0.0
```

Al iniciar Joss, el cargador descubre los paquetes instalados y registra `AI` y `ChatClient` automaticamente.

## Configuracion

Agrega estas variables a `env.joss`:

| Variable | Uso |
| --- | --- |
| `AI_PROVIDER` | Proveedor por defecto: `groq`, `openai` o `gemini`. |
| `AI_MODEL` | Modelo por defecto del proveedor seleccionado. |
| `GROQ_API_KEY` | Clave para Groq. |
| `OPENAI_API_KEY` | Clave para OpenAI. |
| `GEMINI_API_KEY` | Clave para la API compatible de Gemini. |

El proveedor y modelo pueden definirse por llamada, por lo que una misma aplicacion puede usar mas de uno.

## Uso

```joss
$respuesta = AI::client()
    ->provider("groq")
    ->model("llama-3.3-70b-versatile")
    ->system("Responde de forma breve y clara")
    ->user("Explica que es un paquete JP")
    ->call()
```

`AI::chat($provider, $model, $messages)` permite enviar un arreglo de mensajes ya construido. La API fluida expone `provider`, `model`, `system`, `user`, `assistant`, `prompt` y `call`; cada metodo de configuracion devuelve el mismo cliente.

## Streaming

```joss
AI::client()
    ->user("Escribe una bienvenida")
    ->stream(function ($chunk) {
        Console::log($chunk)
    })
```

Para WebSockets, `streamTo($ws)` envia cada fragmento como JSON con la forma `{ "type": "chunk", "content": "..." }`.

## Distribucion y desarrollo

El archivo `joss_ai.jp` es el artefacto distribuible. Su manifiesto declara los seis targets nativos, el bytecode Joss y `META-INF/joss-symbols.json`, usado por la extension de VS Code para autocompletado, firmas y documentacion contextual.

Para reconstruirlo desde este repositorio se requiere Go y Joss 3.6.0 o posterior. El script de distribucion principal compila los sidecars y regenera el JP; los consumidores nunca realizan ese paso.
