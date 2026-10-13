package com.snowflake.carbon;

import static org.assertj.core.api.Assertions.assertThat;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import java.nio.file.Files;
import java.nio.file.Path;

import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.test.web.servlet.MockMvc;

@SpringBootTest
@AutoConfigureMockMvc
class OpenApiExportTest {
    @Autowired
    private MockMvc mockMvc;

    @Autowired
    private ObjectMapper mapper;

    @Test
    void exportsGeneratedContractWithoutExternalServices() throws Exception {
        String json = mockMvc.perform(get("/v3/api-docs"))
                .andExpect(status().isOk())
                .andReturn().getResponse().getContentAsString();
        var spec = mapper.readTree(json);
        assertThat(spec.at("/paths/~1score/get/operationId").asText()).isEqualTo("getScore");
        assertThat(spec.at("/paths/~1score/post/requestBody/description").asText()).isNotBlank();
        assertThat(spec.at("/paths/~1score/get/security/0/Bearer").isArray()).isTrue();

        String outputPath = System.getProperty("openapi.output.path");
        if (outputPath != null) {
            Files.writeString(Path.of(outputPath), json);
        }
    }
}
