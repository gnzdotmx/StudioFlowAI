package chatgpt

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gnzdotmx/studioflowai/studioflowai/internal/modules"
	"github.com/gnzdotmx/studioflowai/studioflowai/internal/utils"

	"gopkg.in/yaml.v3"
)

// SNSModule implements content generation for social network sharing
type SNSModule struct{}

// SNSParams contains the parameters for SNS content generation
type SNSParams struct {
	Input            string  `json:"input"`            // Path to input transcript file
	Output           string  `json:"output"`           // Path to output directory
	InputFileName    string  `json:"inputFileName"`    // Specific input file name to process
	OutputFileName   string  `json:"outputFileName"`   // Custom output file name (without extension)
	Model            string  `json:"model"`            // OpenAI model to use (default: "gpt-4o")
	Temperature      float64 `json:"temperature"`      // Model temperature (default: 0.1)
	MaxTokens        int     `json:"maxTokens"`        // Maximum tokens for the response (default: 8000)
	RequestTimeoutMS int     `json:"requestTimeoutMs"` // API request timeout in milliseconds (default: 120000)
	Language         string  `json:"language"`         // Language for the content (default: "Spanish")
	PromptFilePath   string  `json:"promptFilePath"`   // Path to custom prompt YAML file (default: "./prompts/sns_content.yaml")
}

// New creates a new SNS module
func NewSNS() *SNSModule {
	return &SNSModule{}
}

// Name returns the module name
func (m *SNSModule) Name() string {
	return "sns"
}

// Validate checks if the parameters are valid
func (m *SNSModule) Validate(params map[string]interface{}) error {
	var p SNSParams
	if err := modules.ParseParams(params, &p); err != nil {
		return err
	}

	// Validate input path
	if err := utils.ValidateInputPath(p.Input, p.Output, p.InputFileName); err != nil {
		return err
	}

	// Validate output path
	if err := utils.ValidateOutputPath(p.Output); err != nil {
		return err
	}

	// Check if the API key is set - just warn but don't error
	if os.Getenv("OPENAI_API_KEY") == "" {
		utils.LogWarning("OPENAI_API_KEY environment variable is not set. A placeholder file will be generated.")
	}

	// If a custom prompt file path is provided, check if it exists
	if p.PromptFilePath != "" {
		if _, err := os.Stat(p.PromptFilePath); os.IsNotExist(err) {
			return fmt.Errorf("prompt template file %s does not exist", p.PromptFilePath)
		}
	}

	return nil
}

// Execute generates SNS content using ChatGPT
func (m *SNSModule) Execute(ctx context.Context, params map[string]interface{}) error {
	var p SNSParams
	if err := modules.ParseParams(params, &p); err != nil {
		return err
	}

	// Set default values
	if p.Model == "" {
		p.Model = "gpt-4o"
	}
	if p.Temperature == 0 {
		p.Temperature = 0.1
	}
	if p.MaxTokens == 0 {
		p.MaxTokens = 8000 // Increased for SNS generation
	}
	if p.Language == "" {
		p.Language = "Spanish"
	}
	if p.RequestTimeoutMS == 0 {
		p.RequestTimeoutMS = 120000 // 120 seconds default (increased for larger content)
	}
	if p.PromptFilePath == "" {
		p.PromptFilePath = "./prompts/sns_content.yaml"
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(p.Output, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Get the SNS prompt
	snsPrompt := getSNSPrompt(p.PromptFilePath)

	// Resolve the input path if it contains ${output}
	resolvedInput := utils.ResolveOutputPath(p.Input, p.Output)

	// Verify input exists at execution time (now that previous steps have completed)
	fileInfo, err := os.Stat(resolvedInput)
	if err != nil {
		return fmt.Errorf("input file not found: %w", err)
	}

	if fileInfo.IsDir() {
		return fmt.Errorf("input must be a file, not a directory: %s", resolvedInput)
	}

	// Check if input is a text file
	if !utils.IsTextFile(resolvedInput) {
		return fmt.Errorf("file %s appears to be binary, not a text file", resolvedInput)
	}

	// Determine output file name
	var outputPath string
	if p.OutputFileName != "" {
		outputPath = filepath.Join(p.Output, p.OutputFileName+".yaml")
	} else {
		baseFilename := filepath.Base(resolvedInput)
		baseFilename = baseFilename[:len(baseFilename)-len(filepath.Ext(baseFilename))]
		outputPath = filepath.Join(p.Output, baseFilename+"_SNS.yaml")
	}

	if err := m.processSNSFile(ctx, resolvedInput, outputPath, snsPrompt, p); err != nil {
		return err
	}

	fmt.Println(utils.Success(fmt.Sprintf("Generated SNS content for %s -> %s", resolvedInput, outputPath)))
	return nil
}

// processSNSFile sends a transcript file to ChatGPT for SNS content generation
func (m *SNSModule) processSNSFile(ctx context.Context, inputPath, outputPath, promptTemplate string, p SNSParams) error {
	// Check if the file is a text file
	if !utils.IsTextFile(inputPath) {
		return fmt.Errorf("file %s appears to be binary, not a text file - skipping", inputPath)
	}

	// Read the transcript file
	transcript, err := utils.ReadTextFile(inputPath)
	if err != nil {
		return fmt.Errorf("failed to read transcript file: %w", err)
	}

	// Check if API key is set, if not, save a placeholder file
	if os.Getenv("OPENAI_API_KEY") == "" {
		utils.LogWarning("No API key set - saving placeholder file to %s", outputPath)
		placeholderContent := `# MOCK OUTPUT - No OPENAI_API_KEY set
# Simulated example of generated SNS content in YAML format.

sns_content_generation:
  introduction: "Analiza el siguiente script de entrevista y genera contenido optimizado para maximizar el alcance y engagement en YouTube."

  title: "El secreto detrás del éxito en ciberseguridad | Entrevista exclusiva"

  description:
    🚀 Descubre los secretos que llevaron a nuestro invitado a convertirse en una figura clave de la ciberseguridad. 
    En esta entrevista exclusiva, exploramos su trayectoria, aprendizajes, y consejos para profesionales del sector.
    🔒 Temas clave, historias impactantes y estrategias reales que puedes aplicar hoy.
    
    👉 ¡No olvides suscribirte, dejar tu comentario y compartir este video!
    
    #ciberseguridad #infosec #hackingetico #tecnología #entrevistas

  social_media:
    twitter: "🚨 Nuevo episodio: Entrevista exclusiva sobre ciberseguridad con insights que no te puedes perder 🔐 ¡Dale play ahora! 🎥 #infosec #hackingetico"
    instagram_facebook: >
      🔥 ¡Ya disponible! Entrevistamos a uno de los referentes en ciberseguridad 🎙️ Hablamos sobre sus inicios, retos y cómo ve el futuro del sector. 
      👉 Mira el video completo y comenta qué parte te sorprendió más.
    linkedin: >
      Nueva entrevista publicada con un experto en ciberseguridad. Hablamos sobre tendencias, desafíos y cómo los profesionales pueden adaptarse al entorno actual. 
      Un contenido valioso para quienes lideran equipos de seguridad o aspiran a crecer en esta industria.

  keywords: "ciberseguridad, hacking ético, seguridad informática, entrevistas tecnología, expertos ciberseguridad, SOC, malware, pentesting"

  timeline:
    - "00:00 - Introducción y contexto"
    - "03:15 - Trayectoria profesional del invitado"
    - "10:42 - Principales desafíos en ciberseguridad"
    - "18:20 - Herramientas y consejos prácticos"
    - "25:50 - Futuro del sector"
    - "30:00 - Conclusiones y despedida"

  conclusion: "Este contenido ha sido generado como ejemplo en formato YAML para ilustrar el resultado esperado."  

  transcript_file: "` + inputPath + `"`
		if err := utils.WriteTextFile(outputPath, placeholderContent); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		return nil
	}

	utils.LogVerbose("Generating SNS content for %s...", filepath.Base(inputPath))

	// Create a timeout context for the API request
	apiCtx, cancel := context.WithTimeout(ctx, time.Duration(p.RequestTimeoutMS)*time.Millisecond)
	defer cancel()

	// Construct the full prompt
	fullPrompt := promptTemplate
	if !strings.HasSuffix(fullPrompt, "\n") {
		fullPrompt += "\n\n"
	}
	fullPrompt += "Generar en: " + p.Language + "\n\n"
	fullPrompt += transcript

	// Create the API request
	messages := []ChatMessage{
		{
			Role:    "system",
			Content: "Eres un asistente especializado en optimizar contenido para YouTube, marketing digital y redes sociales. Tu trabajo es analizar transcripciones y generar títulos, descripciones, hashtags y otros contenidos para maximizar visibilidad y engagement.",
		},
		{
			Role:    "user",
			Content: fullPrompt,
		},
	}

	// Send the request to ChatGPT
	response, err := callChatGPT(apiCtx, messages, p.Model, p.Temperature, p.MaxTokens)
	if err != nil {
		return fmt.Errorf("ChatGPT API request failed: %w", err)
	}

	// Write the generated content to the output file
	if err := utils.WriteTextFile(outputPath, response); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	utils.LogSuccess("Generated SNS content for %s -> %s", p.Input, outputPath)
	return nil
}

// getSNSPrompt returns the prompt for SNS content generation
func getSNSPrompt(promptFilePath string) string {
	// Check if a custom prompt template exists
	customPromptPath := promptFilePath
	if _, err := os.Stat(customPromptPath); err == nil {
		// Read the YAML file
		data, err := os.ReadFile(customPromptPath)
		if err == nil {
			// Try to parse as YAML
			yamlPrompt, err := formatSNSYAMLPrompt(data)
			if err == nil {
				utils.LogDebug("Using custom SNS prompt template from YAML file: %s", customPromptPath)
				return yamlPrompt
			}
			utils.LogWarning("Failed to parse YAML prompt: %v, falling back to default", err)
		}
	}

	// Default prompt in markdown format
	utils.LogDebug("Using default SNS prompt template")
	return `Analiza el siguiente script de entrevista y genera contenido optimizado para maximizar el alcance y engagement en YouTube. Por favor proporciona todos los siguientes elementos:

## 1. TÍTULO (50-60 caracteres)
Crea un título impactante y optimizado para SEO que:
- Capture la esencia principal de la entrevista
- Incluya términos de búsqueda relevantes y populares
- Sea conciso pero descriptivo
- Despierte curiosidad e interés inmediato

## 2. DESCRIPCIÓN PARA YOUTUBE (2000 caracteres máx)
Elabora una descripción atractiva que:
- Comience con un gancho poderoso en los primeros 2-3 renglones (visible en la vista previa)
- Resuma los temas principales y aprendizajes clave de la entrevista
- Incluya emojis estratégicamente colocados para mejorar la legibilidad y el atractivo visual
- Incorpore llamadas a la acción claras (suscribirse, comentar, etc.)
- Incluya hashtags relevantes al final (máximo 5-7)
- Presente la información en párrafos cortos con espaciado adecuado

## 3. COPY PARA REDES SOCIALES (3 VERSIONES)
Genera tres versiones diferentes para compartir en redes sociales:
- Una versión corta para Twitter (280 caracteres máx)
- Una versión para Instagram/Facebook (150-200 palabras)
- Una versión para LinkedIn con enfoque profesional (200-250 palabras)
Cada versión debe incluir:
- Los puntos clave más interesantes/controversiales de la entrevista
- Emojis relevantes para aumentar el engagement
- Un gancho fuerte que invite a ver el video completo

## 4. KEYWORDS PARA SEO (25-30 keywords)
Proporciona una lista exhaustiva de palabras clave separadas por coma que:
- Incluya términos de búsqueda de alto volumen relacionados con el tema
- Combine keywords de cola larga y corta
- Incluya variaciones de los términos principales
- Considere términos de tendencia actual relacionados con el tema

## 5. TIMELINE DETALLADO
Crea un timeline completo con marcas de tiempo que:
- Divida el contenido en secciones claras para navegación fácil
- Incluya una breve descripción del tema de cada sección (1-2 líneas)
- Destaque momentos clave/revelaciones importantes
- **IMPORTANTE**: Ajuste las marcas de tiempo considerando que el video está dividido en partes, donde cada parte reinicia en 0:00. Calcula el tiempo acumulado correctamente para cada parte.

Ejemplo:
--------------------------------
Parte 1:
00:00 - Introducción y bienvenida
05:32 - Primer tema importante
...

Parte 2:
00:00 (30:00) - Continuación del tema X
08:45 (38:45) - Nuevo tema Y
...
--------------------------------

Guarda todo el contenido generado con formato YAML.
`
}

// formatSNSYAMLPrompt parses a YAML prompt template for SNS and formats it as text
func formatSNSYAMLPrompt(yamlData []byte) (string, error) {
	var promptData map[string]interface{}
	if err := yaml.Unmarshal(yamlData, &promptData); err != nil {
		return "", fmt.Errorf("failed to parse YAML prompt template: %w", err)
	}

	var result strings.Builder

	// Add introduction if present
	if intro, ok := promptData["introduction"].(string); ok {
		result.WriteString(intro + "\n\n")
	}

	// Process title section
	if title, ok := promptData["title"].(map[string]interface{}); ok {
		result.WriteString("## 1. TÍTULO ")
		if length, ok := title["length"].(string); ok {
			result.WriteString("(" + length + ")\n")
		} else {
			result.WriteString("(50-60 caracteres)\n")
		}

		if desc, ok := title["description"].(string); ok {
			result.WriteString(desc + "\n")
		}

		if criteria, ok := title["criteria"].([]interface{}); ok {
			for _, criterion := range criteria {
				if str, ok := criterion.(string); ok {
					result.WriteString("- " + str + "\n")
				}
			}
		}
		result.WriteString("\n")
	}

	// Process description section
	if desc, ok := promptData["description"].(map[string]interface{}); ok {
		result.WriteString("## 2. DESCRIPCIÓN PARA YOUTUBE ")
		if length, ok := desc["length"].(string); ok {
			result.WriteString("(" + length + ")\n")
		} else {
			result.WriteString("(2000 caracteres máx)\n")
		}

		if desc, ok := desc["description"].(string); ok {
			result.WriteString(desc + "\n")
		}

		if criteria, ok := desc["criteria"].([]interface{}); ok {
			for _, criterion := range criteria {
				if str, ok := criterion.(string); ok {
					result.WriteString("- " + str + "\n")
				}
			}
		}
		result.WriteString("\n")
	}

	// Process social media section
	if social, ok := promptData["social_media"].(map[string]interface{}); ok {
		result.WriteString("## 3. COPY PARA REDES SOCIALES (3 VERSIONES)\n")

		if desc, ok := social["description"].(string); ok {
			result.WriteString(desc + "\n")
		}

		if platforms, ok := social["platforms"].([]interface{}); ok {
			for _, platform := range platforms {
				if str, ok := platform.(string); ok {
					result.WriteString("- " + str + "\n")
				}
			}
		}

		if requirements, ok := social["requirements"].([]interface{}); ok {
			result.WriteString("Cada versión debe incluir:\n")
			for _, req := range requirements {
				if str, ok := req.(string); ok {
					result.WriteString("- " + str + "\n")
				}
			}
		}
		result.WriteString("\n")
	}

	// Process SEO keywords section
	if keywords, ok := promptData["keywords"].(map[string]interface{}); ok {
		result.WriteString("## 4. KEYWORDS PARA SEO ")
		if count, ok := keywords["count"].(string); ok {
			result.WriteString("(" + count + ")\n")
		} else {
			result.WriteString("(25-30 keywords)\n")
		}

		if desc, ok := keywords["description"].(string); ok {
			result.WriteString(desc + "\n")
		}

		if criteria, ok := keywords["criteria"].([]interface{}); ok {
			for _, criterion := range criteria {
				if str, ok := criterion.(string); ok {
					result.WriteString("- " + str + "\n")
				}
			}
		}
		result.WriteString("\n")
	}

	// Process timeline section
	if timeline, ok := promptData["timeline"].(map[string]interface{}); ok {
		result.WriteString("## 5. TIMELINE DETALLADO\n")

		if desc, ok := timeline["description"].(string); ok {
			result.WriteString(desc + "\n")
		}

		if criteria, ok := timeline["criteria"].([]interface{}); ok {
			for _, criterion := range criteria {
				if str, ok := criterion.(string); ok {
					result.WriteString("- " + str + "\n")
				}
			}
		}

		if example, ok := timeline["example"].(string); ok {
			result.WriteString("\nEjemplo:\n--------------------------------\n")
			result.WriteString(example + "\n")
			result.WriteString("--------------------------------\n\n")
		}
	}

	// Add final instruction
	if conclusion, ok := promptData["conclusion"].(string); ok {
		result.WriteString(conclusion + "\n")
	}

	return result.String(), nil
}

// callChatGPT sends a request to the OpenAI API with reuse of the existing function
func callChatGPT(ctx context.Context, messages []ChatMessage, model string, temperature float64, maxTokens int) (string, error) {
	// Create a ChatGPT module to reuse existing code
	module := &Module{}

	// Get API key from environment
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("OPENAI_API_KEY environment variable is not set")
	}

	// Use the existing callChatGPT implementation
	chatGPTParams := Params{
		Model:       model,
		Temperature: temperature,
		MaxTokens:   maxTokens,
	}

	return module.callChatGPT(ctx, messages, chatGPTParams)
}
