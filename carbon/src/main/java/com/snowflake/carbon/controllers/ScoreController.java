package com.snowflake.carbon.controllers;

import com.snowflake.carbon.entities.Score;
import com.snowflake.carbon.model.response.ScoreNotFoundResponse;
import com.snowflake.carbon.model.response.ValidationErrorResponse;
import com.snowflake.carbon.model.request.CalculateScoreRequest;
import com.snowflake.carbon.model.response.ErrorResponse;
import com.snowflake.carbon.model.response.ScoreResponse;
import com.snowflake.carbon.services.ScoreService;
import io.swagger.v3.oas.annotations.Operation;
import io.swagger.v3.oas.annotations.security.SecurityRequirement;
import io.swagger.v3.oas.annotations.media.Content;
import io.swagger.v3.oas.annotations.media.ExampleObject;
import io.swagger.v3.oas.annotations.media.Schema;
import io.swagger.v3.oas.annotations.responses.ApiResponse;
import io.swagger.v3.oas.annotations.responses.ApiResponses;
import io.swagger.v3.oas.annotations.tags.Tag;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.validation.Valid;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RestController;

@SecurityRequirement(name = "Bearer")
@Tag(name = "Scorer", description = "Operations related to credit score")
@RestController
public class ScoreController {

    @Autowired
    private ScoreService scoreService;

    @Operation(operationId = "getScore", summary = "Get credit score", description = "Read the authenticated user’s latest credit score.")
    @ApiResponses(value = {
            @ApiResponse(
                    responseCode = "200",
                    description = "ScoreResponse",
                    content = @Content(
                            schema = @Schema(implementation = ScoreResponse.class),
                            mediaType = "application/json",
                            examples = @ExampleObject(value = "{ \"creditScore\": 750 }")
                    )
            ),
            @ApiResponse(
                    responseCode = "404",
                    description = "Score not found",
                    content = @Content(
                            schema = @Schema(implementation = ScoreNotFoundResponse.class),
                            mediaType = "application/json",
                            examples = @ExampleObject(
                                    name = "ScoreNotFoundResponse",
                                    value = "{ \"message\": \"Score not found\", \"status_code\": 404 }"
                            )
                    )
            ),
            @ApiResponse(
                    responseCode = "500",
                    description = "Internal server error",
                    content = @Content(
                            schema = @Schema(implementation = ErrorResponse.class),
                            mediaType = "application/json",
                            examples = @ExampleObject(
                                    name = "ErrorResponse",
                                    value = "{ \"message\": \"Something went wrong :(\", \"status_code\": 500 }")
                    )
            )
    })
    @GetMapping("/score")
    public ResponseEntity<ScoreResponse> getScore(HttpServletRequest request) {
        String userId = request.getAttribute("userId").toString();

        Score score = scoreService.getScore(userId);

        return ResponseEntity.ok(
                new ScoreResponse(
                        score.getCreditScore()
                )
        );
    }

    @Operation(operationId = "calculateScore", summary = "Calculate credit score", description = "Recalculate the authenticated user’s credit score from their financial profile.")
    @ApiResponses(value = {
            @ApiResponse(
                    responseCode = "200",
                    description = "OK",
                    content = @Content(
                            schema = @Schema(implementation = ScoreResponse.class),
                            mediaType = "application/json",
                            examples = @ExampleObject(value = "{ \"creditScore\": 750 }")
                    )
            ),
            @ApiResponse(
                    responseCode = "400",
                    description = "Invalid request body",
                    content = @Content(
                            schema = @Schema(implementation = ValidationErrorResponse.class),
                            mediaType = "application/json",
                            examples = @ExampleObject(
                                    name = "ValidationErrorResponse",
                                    value = "{ \"message\": \"Invalid request body\", \"status_code\": 400, \"errors\": { \"income\": \"Income must be greater than zero.\", \"debt\": \"Debt cannot be null.\", \"assetsValue\": \"Assets value cannot be negative.\" } }"
                            )
                    )
            ),
            @ApiResponse(
                    responseCode = "500",
                    description = "Internal server error",
                    content = @Content(
                            schema = @Schema(implementation = ErrorResponse.class),
                            mediaType = "application/json",
                            examples = @ExampleObject(
                                    name = "ErrorResponse",
                                    value = "{ \"message\": \"Something went wrong :(\", \"status_code\": 500 }")
                    )
            )
    })
    @PostMapping("/score")
    public ResponseEntity<ScoreResponse> calculateScore(HttpServletRequest request, @io.swagger.v3.oas.annotations.parameters.RequestBody(description = "Annual income, debt and assets value in USD used to calculate the credit score.") @Valid @RequestBody CalculateScoreRequest dto) {

        String userId = request.getAttribute("userId").toString();

        Score score = scoreService.calculateScore(userId, dto.incomeAsDecimal(), dto.debtAsDecimal(), dto.assetsValueAsDecimal());

        return ResponseEntity.ok(
                new ScoreResponse(
                        score.getCreditScore()
                )
        );
    }
}
