using System.Text.Json.Serialization;

namespace Oxygen.DTO.Response;

public class ValidationErrorResponse
{
    /// <summary>
    /// Error message
    /// </summary>
    /// <example>Invalid request body.</example>
    public required string Message { get; set; }
    
    /// <summary>
    /// HTTP status code for the loan validation failure
    /// </summary>
    /// <example>400</example>
    [JsonPropertyName("status_code")]
    public required int StatusCode { get; set; }
    
    /// <summary>
    /// Loan request fields and their validation failures
    /// </summary>
    /// <example>{"loanAmount": "The LoanAmount field is required.", "term": "Term must be between 1 and 60." }</example>
    public required Dictionary<string, string> Errors { get; set; }
}
