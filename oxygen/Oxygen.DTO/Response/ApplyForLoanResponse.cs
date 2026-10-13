using Oxygen.Domain.Enums;

namespace Oxygen.DTO.Response;

public class ApplyForLoanResponse
{
    /// <summary> Loan application status </summary>
    /// <example>APPROVED</example>
    public LoanApplicationStatus Status { get; set; }
    
    /// <summary> Message </summary>
    /// <example>Loan approved :)</example>
    public required string Message { get; set; }
    
    /// <summary> Loan amount </summary>
    /// <example>1000.00</example>
    public decimal Amount { get; set; }
    
    /// <summary> Loan term in months</summary>
    /// <example>12</example>
    public int Term { get; set; }
    
    /// <summary> Final annual interest rate applied </summary>
    /// <example>5.0</example>
    public decimal? InterestRate { get; set; }

    /// <summary> Monthly payment amount </summary>
    /// <example>456.78</example>
    public decimal? MonthlyPayment { get; set; }

    /// <summary> Total payment over the life of the loan </summary>
    /// <example>5481.36</example>
    public decimal? TotalPayment { get; set; }
}
