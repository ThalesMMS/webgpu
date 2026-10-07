package wgpu

import (
	"testing"

	"github.com/gogpu/gputypes"
)

// TestCreateRenderPipelineWithVertexAttributes passes vertex attributes through
// the v29 WGPUVertexAttribute layout; a misaligned wire struct makes
// wgpu-native read the wrong format/offset/location and reject the pipeline.
func TestCreateRenderPipelineWithVertexAttributes(t *testing.T) {
	inst, err := CreateInstance(nil)
	if err != nil {
		t.Fatalf("CreateInstance failed: %v", err)
	}
	defer inst.Release()

	adapter, err := inst.RequestAdapter(nil)
	if err != nil {
		t.Fatalf("RequestAdapter failed: %v", err)
	}
	defer adapter.Release()

	device, err := adapter.RequestDevice(nil)
	if err != nil {
		t.Fatalf("RequestDevice failed: %v", err)
	}
	defer device.Release()

	shaderCode := `
struct VertexInput {
    @location(0) position: vec2<f32>,
    @location(1) color: vec3<f32>,
}

struct VertexOutput {
    @builtin(position) position: vec4<f32>,
    @location(0) color: vec3<f32>,
}

@vertex
fn vs_main(in: VertexInput) -> VertexOutput {
    var out: VertexOutput;
    out.position = vec4<f32>(in.position, 0.0, 1.0);
    out.color = in.color;
    return out;
}

@fragment
fn fs_main(in: VertexOutput) -> @location(0) vec4<f32> {
    return vec4<f32>(in.color, 1.0);
}
`
	shader, err := device.CreateShaderModuleWGSL(shaderCode)
	if err != nil {
		t.Fatalf("CreateShaderModuleWGSL: %v", err)
	}
	defer shader.Release()

	attributes := []VertexAttribute{
		{Format: gputypes.VertexFormatFloat32x2, Offset: 0, ShaderLocation: 0},
		{Format: gputypes.VertexFormatFloat32x3, Offset: 8, ShaderLocation: 1},
	}
	pipeline, err := device.CreateRenderPipeline(&RenderPipelineDescriptor{
		Vertex: VertexState{
			Module:     shader,
			EntryPoint: "vs_main",
			Buffers: []VertexBufferLayout{{
				ArrayStride:    20,
				StepMode:       gputypes.VertexStepModeVertex,
				AttributeCount: uintptr(len(attributes)),
				Attributes:     &attributes[0],
			}},
		},
		Fragment: &FragmentState{
			Module:     shader,
			EntryPoint: "fs_main",
			Targets: []ColorTargetState{{
				Format:    gputypes.TextureFormatBGRA8Unorm,
				WriteMask: gputypes.ColorWriteMaskAll,
			}},
		},
		Primitive: PrimitiveState{
			Topology:  gputypes.PrimitiveTopologyTriangleList,
			FrontFace: gputypes.FrontFaceCCW,
			CullMode:  gputypes.CullModeNone,
		},
		Multisample: MultisampleState{
			Count: 1,
			Mask:  0xFFFFFFFF,
		},
	})
	if err != nil {
		t.Fatalf("CreateRenderPipeline: %v", err)
	}
	defer pipeline.Release()

	if pipeline.Handle() == 0 {
		t.Fatal("RenderPipeline handle is zero")
	}
}
